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
	"gorm.io/gorm/clause"
)

var (
	forUpdateClause = clause.Locking{Strength: "UPDATE", Options: "NOWAIT"}
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

func (orchestrator *storeOrchestrator) GetEpisodeWithTmdbId(tmdbID string) (*media.Episode, error) {
	return orchestrator.MediaStore.GetEpisodeWithTmdbId(orchestrator.db.GetInstance(), tmdbID)
}

func (orchestrator *storeOrchestrator) GetSeason(seasonId uuid.UUID) (*media.Season, error) {
	return orchestrator.MediaStore.GetSeason(orchestrator.db.GetInstance(), seasonId)
}

func (orchestrator *storeOrchestrator) GetSeasonWithTmdbId(tmdbID string) (*media.Season, error) {
	return orchestrator.MediaStore.GetSeasonWithTmdbId(orchestrator.db.GetInstance(), tmdbID)
}

func (orchestrator *storeOrchestrator) GetSeries(seriesId uuid.UUID) (*media.Series, error) {
	return orchestrator.MediaStore.GetSeries(orchestrator.db.GetInstance(), seriesId)
}

func (orchestrator *storeOrchestrator) GetSeriesWithTmdbId(tmdbID string) (*media.Series, error) {
	return orchestrator.MediaStore.GetSeriesWithTmdbId(orchestrator.db.GetInstance(), tmdbID)
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
//
// Note: If the season/series are not provided, and the FK-constraint of the episode cannot
// be fulfilled because of this, then the save will fail. It is recommended to supply all parameters.
func (orchestrator *storeOrchestrator) SaveEpisode(episode *media.Episode, season *media.Season, series *media.Series) error {
	// Store old PKs so we can rollback on transaction failure
	episodeId := episode.ID
	seasonId := season.ID
	seriesId := series.ID

	if err := orchestrator.db.GetInstance().Transaction(func(tx *gorm.DB) error {
		if err := orchestrator.MediaStore.SaveSeries(tx, series); err != nil {
			return err
		}

		if err := orchestrator.MediaStore.SaveSeason(tx, season); err != nil {
			return err
		}

		var existingEpisode *media.Episode
		tx.Where(&media.Episode{Model: media.Model{TmdbId: episode.TmdbId}}).First(&existingEpisode)
		if existingEpisode != nil {
			episode.ID = existingEpisode.ID
		}

		err := tx.Debug().Save(episode).Error
		if err != nil {
			episode.ID = episodeId
			return err
		}

		return nil
	}); err != nil {
		episode.ID = episodeId
		season.ID = seasonId
		series.ID = seriesId

		return err
	}

	return nil
}

// Workflows

// CreateWorkflow uses the information provided to construct and save a new workflow
// in a single DB transaction.
//
// Error will be returned if any of the target IDs provided do not refer to existing Target
// DB entries, or if the workflow infringes on any uniqueness constraints (label)
func (orchestrator *storeOrchestrator) CreateWorkflow(workflowID uuid.UUID, label string, criteria []match.Criteria, targetIDs []uuid.UUID, enabled bool) (*workflow.Workflow, error) {
	var newWorkflow *workflow.Workflow
	if txErr := orchestrator.db.GetInstance().Transaction(func(tx *gorm.DB) error {
		targetModels := orchestrator.TargetStore.GetMany(tx, targetIDs...)
		if len(targetModels) != len(targetIDs) {
			return fmt.Errorf("target IDs %v reference one or more missing targets", targetIDs)
		}

		newWorkflow = &workflow.Workflow{ID: workflowID, Label: label, Criteria: criteria, Targets: targetModels}
		if err := orchestrator.WorkflowStore.Create(tx, newWorkflow); err != nil {
			return err
		}

		return nil
	}); txErr == nil {
		return newWorkflow, nil
	} else {
		return nil, txErr
	}
}

// UpdateWorkflow transactionally updates an existing Workflow model
// using the optional paramaters provided. If a param is `nil` then the
// corresponding value in the model is NOT changed.
func (orchestrator *storeOrchestrator) UpdateWorkflow(workflowID uuid.UUID, newLabel *string, newCriteria *[]match.Criteria, newTargetIDs *[]uuid.UUID, newEnabled *bool) (*workflow.Workflow, error) {
	var outputWorkflow *workflow.Workflow
	if txErr := orchestrator.db.GetInstance().Debug().Transaction(func(tx *gorm.DB) error {
		var existingWorkflow *workflow.Workflow = nil

		if err := tx.Clauses(forUpdateClause).Where(workflow.Workflow{ID: workflowID}).First(&existingWorkflow).Error; err != nil {
			return fmt.Errorf("failed to find workflow with ID = %s due to error: %s", workflowID, err.Error())
		} else if existingWorkflow == nil {
			return fmt.Errorf("failed to find workflow with ID = %s", workflowID)
		}

		if newTargetIDs != nil {
			targetModels := orchestrator.TargetStore.GetMany(tx, *newTargetIDs...)
			if len(targetModels) != len(*newTargetIDs) {
				return fmt.Errorf("target IDs %v reference one or more missing targets", *newTargetIDs)
			}

			if err := tx.Debug().Model(&existingWorkflow).Association("Targets").Replace(targetModels); err != nil {
				return fmt.Errorf("failed to update workflow target associations due to error %s", err.Error())
			}
		}

		if newCriteria != nil {
			if err := tx.Debug().Model(&existingWorkflow).Association("Criteria").Unscoped().Replace(newCriteria); err != nil {
				return fmt.Errorf("failed to update workflow criteria associations due to error %s", err.Error())
			}
		}

		columnUpdates := make(map[string]any)
		if newLabel != nil {
			columnUpdates["label"] = newLabel
		}
		if newEnabled != nil {
			columnUpdates["enabled"] = newEnabled
		}

		if err := tx.Debug().Model(&existingWorkflow).Updates(columnUpdates).Error; err != nil {
			return fmt.Errorf("failed to update workflow row due to error %s", err.Error())
		}

		outputWorkflow = orchestrator.WorkflowStore.Get(tx, workflowID)
		return nil
	}); txErr == nil {
		return outputWorkflow, nil
	} else {
		return nil, txErr
	}
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
func (orchestrator *storeOrchestrator) GetTranscode(id uuid.UUID) *transcode.TranscodeTask {
	return orchestrator.TranscodeStore.Get(orchestrator.db.GetInstance(), id)
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
