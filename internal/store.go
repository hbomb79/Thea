package internal

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/internal/transcode"
	"github.com/hbomb79/Thea/internal/workflow"
	"github.com/hbomb79/Thea/internal/workflow/match"
	"gorm.io/gorm"
)

type (
	// storeOrchestrator is responsible for managing all of Thea's resources,
	// especially highly-relational data. You can think of all
	// the data stores below this layer being 'dumb', and this store
	// linking them together and providing the database instance
	//
	// If consumers need to be able to access data stores directly, they're
	// welcome to do so - however caution should be taken as stores have no
	// obligation to take care of relational data (which is the orchestrator's job)
	storeOrchestrator struct {
		db             database.Manager
		MediaStore     *media.Store
		TranscodeStore *transcode.Store
		WorkflowStore  *workflow.Store
		TargetStore    *ffmpeg.Store
	}
)

func NewStoreOrchestrator(db database.Manager) (*storeOrchestrator, error) {
	if db.GetInstance() != nil {
		panic("cannot construct thea data store with an already connected database")
	}

	store := &storeOrchestrator{
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

func (orchestrator *storeOrchestrator) GetMedia(mediaId uuid.UUID) *media.Container {
	return orchestrator.MediaStore.GetMedia(orchestrator.db.GetInstance(), mediaId)
}

func (orchestrator *storeOrchestrator) GetMovie(movieId uuid.UUID) (*media.Movie, error) {
	return orchestrator.MediaStore.GetMovie(orchestrator.db.GetInstance(), movieId)
}

func (orchestrator *storeOrchestrator) GetEpisode(episodeId uuid.UUID) (*media.Episode, error) {
	return orchestrator.MediaStore.GetEpisode(orchestrator.db.GetInstance(), episodeId)
}

func (orchestrator *storeOrchestrator) GetSeason(seasonId uuid.UUID) (*media.Season, error) {
	return orchestrator.MediaStore.GetSeason(orchestrator.db.GetInstance(), seasonId)
}

func (orchestrator *storeOrchestrator) GetSeries(seriesId uuid.UUID) (*media.Series, error) {
	return orchestrator.MediaStore.GetSeries(orchestrator.db.GetInstance(), seriesId)
}

func (orchestrator *storeOrchestrator) GetAllMediaSourcePaths() []string {
	return orchestrator.MediaStore.GetAllSourcePaths(orchestrator.db.GetInstance())
}

func (orchestrator *storeOrchestrator) SaveMovie(movie *media.Movie) error {
	return orchestrator.MediaStore.SaveMovie(orchestrator.db.GetInstance(), movie)
}

func (orchestrator *storeOrchestrator) SaveSeries(series *media.Series) error {
	return orchestrator.MediaStore.SaveSeries(orchestrator.db.GetInstance(), series)
}

func (orchestrator *storeOrchestrator) SaveSeason(season *media.Season) error {
	return orchestrator.MediaStore.SaveSeason(orchestrator.db.GetInstance(), season)
}

// SaveEpisode transactoinally saves the episode provided, as well as the season and series
// it's associatted with IF they are provided.
func (orchestrator *storeOrchestrator) SaveEpisode(episode *media.Episode, season *media.Season, series *media.Series) error {
	// Store old PKs so we can rollback on transaction failure
	episodeId := episode.Id
	seasonId := season.Id
	seriesId := series.Id

	if err := orchestrator.db.GetInstance().Transaction(func(tx *gorm.DB) error {
		if err := orchestrator.MediaStore.SaveSeries(tx, series); err != nil {
			return err
		}

		if err := orchestrator.MediaStore.SaveSeason(tx, season); err != nil {
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

// CreateWorkflow uses the information provided to construct and save a new workflow
// in a single DB transaction.
func (orchestrator *storeOrchestrator) CreateWorkflow(workflowID uuid.UUID, label string, criteria []match.Criteria, targetIDs []uuid.UUID) (*workflow.Workflow, error) {
	var newWorkflow *workflow.Workflow
	if txErr := orchestrator.db.GetInstance().Transaction(func(tx *gorm.DB) error {
		var targetModels []*ffmpeg.Target
		if err := tx.Find(&targetModels, targetIDs).Error; err != nil {
			return fmt.Errorf("target IDs %v could not be resolved to matching targets", err.Error())
		}

		newWorkflow = &workflow.Workflow{
			ID:       workflowID,
			Label:    label,
			Criteria: criteria,
			Targets:  targetModels,
		}

		if err := orchestrator.WorkflowStore.Save(tx, newWorkflow); err != nil {
			return err
		}

		return nil
	}); txErr == nil {
		return newWorkflow, nil
	} else {
		return nil, txErr
	}
}

func (orchestrator *storeOrchestrator) SaveWorkflow(workflow *workflow.Workflow) error {
	return orchestrator.WorkflowStore.Save(orchestrator.db.GetInstance(), workflow)
}

func (orchestrator *storeOrchestrator) GetWorkflow(id uuid.UUID) *workflow.Workflow {
	return orchestrator.WorkflowStore.Get(orchestrator.db.GetInstance(), id)
}

func (orchestrator *storeOrchestrator) GetAllWorkflows() []*workflow.Workflow {
	return orchestrator.WorkflowStore.GetAll(orchestrator.db.GetInstance())
}

func (orchestrator *storeOrchestrator) DeleteWorkflow(id uuid.UUID) {
	orchestrator.WorkflowStore.Delete(orchestrator.db.GetInstance(), id)
}

// Transcodes

func (orchestrator *storeOrchestrator) SaveTranscode(transcode *transcode.TranscodeTask) error {
	return orchestrator.TranscodeStore.SaveTranscode(orchestrator.db.GetInstance(), transcode)
}
func (orchestrator *storeOrchestrator) GetAllTranscodes() ([]*transcode.TranscodeTask, error) {
	return orchestrator.TranscodeStore.GetAll(orchestrator.db.GetInstance())
}
func (orchestrator *storeOrchestrator) GetTranscodesForMedia(mediaId uuid.UUID) ([]*transcode.TranscodeTask, error) {
	return orchestrator.TranscodeStore.GetForMedia(orchestrator.db.GetInstance(), mediaId)
}

// Targets

func (orchestrator *storeOrchestrator) SaveTarget(target *ffmpeg.Target) error {
	return orchestrator.TargetStore.Save(orchestrator.db.GetInstance(), target)
}

func (orchestrator *storeOrchestrator) GetTarget(id uuid.UUID) *ffmpeg.Target {
	return orchestrator.TargetStore.Get(orchestrator.db.GetInstance(), id)
}

func (orchestrator *storeOrchestrator) GetAllTargets() []*ffmpeg.Target {
	return orchestrator.TargetStore.GetAll(orchestrator.db.GetInstance())
}

func (orchestrator *storeOrchestrator) GetManyTargets(ids ...uuid.UUID) []*ffmpeg.Target {
	return orchestrator.TargetStore.GetMany(orchestrator.db.GetInstance(), ids...)
}

func (orchestrator *storeOrchestrator) DeleteTarget(id uuid.UUID) {
	orchestrator.TargetStore.Delete(orchestrator.db.GetInstance(), id)
}
