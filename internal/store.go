package internal

import (
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/internal/transcode"
	"github.com/hbomb79/Thea/internal/workflow"
	"gorm.io/gorm"
)

type (
	// dataOrchestrator is responsible for managing all of Thea's resources,
	// especially highly-relational data. You can think of all
	// the data stores below this layer being 'dumb', and this store
	// linking them together and providing the database instance
	//
	// If consumers need to be able to access data stores directly, they're
	// welcome to do so - however caution should be taken as stores have no
	// obligation to take care of relational data (which is the orchestrator's job)
	dataOrchestrator struct {
		db             database.Manager
		MediaStore     *media.Store
		TranscodeStore *transcode.Store
		WorkflowStore  *workflow.Store
		TargetStore    *ffmpeg.Store
	}
)

func NewDataOrchestrator(db database.Manager) (*dataOrchestrator, error) {
	if db.GetInstance() != nil {
		panic("cannot construct thea data store with an already connected database")
	}

	store := &dataOrchestrator{
		db:             db,
		MediaStore:     &media.Store{},
		TranscodeStore: &transcode.Store{},
		WorkflowStore:  &workflow.Store{},
		TargetStore:    &ffmpeg.Store{},
	}

	store.MediaStore.RegisterModels(db)
	store.TranscodeStore.RegisterModels(db)
	store.WorkflowStore.RegisterModels(db)
	store.TargetStore.RegisterModels(db)

	return store, nil
}

func (rel *dataOrchestrator) GetMedia(mediaId uuid.UUID) *media.Container {
	return rel.MediaStore.GetMedia(rel.db.GetInstance(), mediaId)
}

func (rel *dataOrchestrator) GetMovie(movieId uuid.UUID) (*media.Movie, error) {
	return rel.MediaStore.GetMovie(rel.db.GetInstance(), movieId)
}

func (rel *dataOrchestrator) GetEpisode(episodeId uuid.UUID) (*media.Episode, error) {
	return rel.MediaStore.GetEpisode(rel.db.GetInstance(), episodeId)
}

func (rel *dataOrchestrator) GetSeason(seasonId uuid.UUID) (*media.Season, error) {
	return rel.MediaStore.GetSeason(rel.db.GetInstance(), seasonId)
}

func (rel *dataOrchestrator) GetSeries(seriesId uuid.UUID) (*media.Series, error) {
	return rel.MediaStore.GetSeries(rel.db.GetInstance(), seriesId)
}

func (rel *dataOrchestrator) GetAllMediaSourcePaths() []string {
	return rel.MediaStore.GetAllSourcePaths(rel.db.GetInstance())
}

func (rel *dataOrchestrator) SaveMovie(movie *media.Movie) error {
	return rel.MediaStore.SaveMovie(rel.db.GetInstance(), movie)
}

func (rel *dataOrchestrator) SaveSeries(series *media.Series) error {
	return rel.MediaStore.SaveSeries(rel.db.GetInstance(), series)
}

func (rel *dataOrchestrator) SaveSeason(season *media.Season) error {
	return rel.MediaStore.SaveSeason(rel.db.GetInstance(), season)
}

// SaveEpisode transactoinally saves the episode provided, as well as the season and series
// it's associatted with IF they are provided.
func (rel *dataOrchestrator) SaveEpisode(episode *media.Episode, season *media.Season, series *media.Series) error {
	// Store old PKs so we can rollback on transaction failure
	episodeId := episode.Id
	seasonId := season.Id
	seriesId := series.Id

	if err := rel.db.GetInstance().Transaction(func(tx *gorm.DB) error {
		if err := rel.MediaStore.SaveSeries(tx, series); err != nil {
			return err
		}

		if err := rel.MediaStore.SaveSeason(tx, season); err != nil {
			return err
		}

		var existingEpisode *media.Episode
		tx.Where(&media.Episode{Common: media.Common{TmdbId: episode.TmdbId}}).First(&existingEpisode)
		if existingEpisode != nil {
			episode.Id = existingEpisode.Id
		}

		err := tx.Debug().Save(episode).Error
		if err != nil {
			episode.Id = episodeId
			return err
		}

		return nil
	}); err != nil {
		episode.Id = episodeId
		season.Id = seasonId
		series.Id = seriesId

		return err
	}

	return nil
}

// Workflows

func (data *dataOrchestrator) SaveWorkflow(workflow *workflow.Workflow) error {
	return data.WorkflowStore.Save(data.db.GetInstance(), workflow)
}

func (data *dataOrchestrator) GetWorkflow(id uuid.UUID) *workflow.Workflow {
	return data.WorkflowStore.Get(data.db.GetInstance(), id)
}

func (data *dataOrchestrator) GetAllWorkflows() []*workflow.Workflow {
	return data.WorkflowStore.GetAll(data.db.GetInstance())
}

func (data *dataOrchestrator) DeleteWorkflow(id uuid.UUID) {
	data.WorkflowStore.Delete(data.db.GetInstance(), id)
}

// Transcodes

func (data *dataOrchestrator) SaveTranscode(transcode *transcode.TranscodeTask) error {
	return data.TranscodeStore.SaveTranscode(data.db.GetInstance(), transcode)
}
func (data *dataOrchestrator) GetAllTranscodes() ([]*transcode.TranscodeTask, error) {
	return data.TranscodeStore.GetAll(data.db.GetInstance())
}
func (data *dataOrchestrator) GetTranscodesForMedia(mediaId uuid.UUID) ([]*transcode.TranscodeTask, error) {
	return data.TranscodeStore.GetForMedia(data.db.GetInstance(), mediaId)
}

// Targets

func (data *dataOrchestrator) SaveTarget(target *ffmpeg.Target) error {
	return data.TargetStore.Save(data.db.GetInstance(), target)
}

func (data *dataOrchestrator) GetTarget(id uuid.UUID) *ffmpeg.Target {
	return data.TargetStore.Get(data.db.GetInstance(), id)
}

func (data *dataOrchestrator) GetAllTargets() []*ffmpeg.Target {
	return data.TargetStore.GetAll(data.db.GetInstance())
}

func (data *dataOrchestrator) GetManyTargets(ids ...uuid.UUID) []*ffmpeg.Target {
	return data.TargetStore.GetMany(data.db.GetInstance(), ids...)
}

func (data *dataOrchestrator) DeleteTarget(id uuid.UUID) {
	data.TargetStore.Delete(data.db.GetInstance(), id)
}
