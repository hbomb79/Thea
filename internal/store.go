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
	"github.com/jmoiron/sqlx"
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
	if db.GetSqlxDb() != nil {
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
	return orchestrator.MediaStore.GetMedia(orchestrator.db.GetSqlxDb(), mediaId)
}

func (orchestrator *storeOrchestrator) GetMovie(movieId uuid.UUID) (*media.Movie, error) {
	return orchestrator.MediaStore.GetMovie(orchestrator.db.GetSqlxDb(), movieId)
}

func (orchestrator *storeOrchestrator) GetEpisode(episodeId uuid.UUID) (*media.Episode, error) {
	return orchestrator.MediaStore.GetEpisode(orchestrator.db.GetSqlxDb(), episodeId)
}

func (orchestrator *storeOrchestrator) GetEpisodeWithTmdbId(tmdbID string) (*media.Episode, error) {
	return orchestrator.MediaStore.GetEpisodeWithTmdbId(orchestrator.db.GetSqlxDb(), tmdbID)
}

func (orchestrator *storeOrchestrator) GetSeason(seasonId uuid.UUID) (*media.Season, error) {
	return orchestrator.MediaStore.GetSeason(orchestrator.db.GetSqlxDb(), seasonId)
}

func (orchestrator *storeOrchestrator) GetSeasonWithTmdbId(tmdbID string) (*media.Season, error) {
	return orchestrator.MediaStore.GetSeasonWithTmdbId(orchestrator.db.GetSqlxDb(), tmdbID)
}

func (orchestrator *storeOrchestrator) GetSeries(seriesId uuid.UUID) (*media.Series, error) {
	return orchestrator.MediaStore.GetSeries(orchestrator.db.GetSqlxDb(), seriesId)
}

func (orchestrator *storeOrchestrator) GetSeriesWithTmdbId(tmdbID string) (*media.Series, error) {
	return orchestrator.MediaStore.GetSeriesWithTmdbId(orchestrator.db.GetSqlxDb(), tmdbID)
}

func (orchestrator *storeOrchestrator) GetAllMediaSourcePaths() []string {
	return orchestrator.MediaStore.GetAllSourcePaths(orchestrator.db.GetSqlxDb())
}

func (orchestrator *storeOrchestrator) SaveMovie(movie *media.Movie) error {
	return orchestrator.MediaStore.SaveMovie(orchestrator.db.GetSqlxDb(), movie)
}

func (orchestrator *storeOrchestrator) SaveSeries(series *media.Series) error {
	return orchestrator.MediaStore.SaveSeries(orchestrator.db.GetSqlxDb(), series)
}

func (orchestrator *storeOrchestrator) SaveSeason(season *media.Season) error {
	return orchestrator.MediaStore.SaveSeason(orchestrator.db.GetSqlxDb(), season)
}

// SaveEpisode transactoinally saves the episode provided, as well as the season and series
// it's associatted with IF they are provided. The relational FK's of the series/season
// will automatically be set to the new/existing DB models.
//
// Note: If the season/series are not provided, and the FK-constraint of the episode cannot
// be fulfilled because of this, then the save will fail. It is recommended to supply all parameters.
func (orchestrator *storeOrchestrator) SaveEpisode(episode *media.Episode, season *media.Season, series *media.Series) error {
	// Store old PKs so we can rollback on transaction failure
	// episodeId := episode.ID
	// seasonId := season.ID
	// seriesId := series.ID

	// if err := orchestrator.db.GetGoquDb().WithTx(func(tx *goqu.TxDatabase) error {
	// 	log.Emit(logger.WARNING, "Saving episode (ID=%s), series (ID=%s), season (ID=%s)\n", episode.ID.String(), series.ID.String(), season.ID.String())
	// 	if err := orchestrator.MediaStore.SaveSeries(tx, series); err != nil {
	// 		return err
	// 	}

	// 	if err := orchestrator.MediaStore.SaveSeason(tx, season); err != nil {
	// 		return err
	// 	}

	// 	return orchestrator.MediaStore.SaveEpisode(tx, episode)
	// }); err != nil {
	// 	episode.ID = episodeId
	// 	season.ID = seasonId
	// 	series.ID = seriesId

	// 	return err
	// }

	return nil
}

// Workflows

// CreateWorkflow uses the information provided to construct and save a new workflow
// in a single DB transaction.
//
// Error will be returned if any of the target IDs provided do not refer to existing Target
// DB entries, or if the workflow infringes on any uniqueness constraints (label)
func (orchestrator *storeOrchestrator) CreateWorkflow(workflowID uuid.UUID, label string, criteria []match.Criteria, targetIDs []uuid.UUID, enabled bool) (*workflow.Workflow, error) {
	db := orchestrator.db.GetSqlxDb()
	if err := orchestrator.WorkflowStore.Create(db, workflowID, label, enabled, targetIDs, criteria); err != nil {
		return nil, err
	}

	return orchestrator.WorkflowStore.Get(db, workflowID), nil
}

// UpdateWorkflow transactionally updates an existing Workflow model
// using the optional paramaters provided. If a param is `nil` then the
// corresponding value in the model is NOT changed.
func (orchestrator *storeOrchestrator) UpdateWorkflow(workflowID uuid.UUID, newLabel *string, newCriteria *[]match.Criteria, newTargetIDs *[]uuid.UUID, newEnabled *bool) (*workflow.Workflow, error) {
	fail := func(desc string, err error) (*workflow.Workflow, error) {
		return nil, fmt.Errorf("failed to %s due to error: %s", desc, err.Error())
	}

	tx, err := orchestrator.db.GetSqlxDb().Beginx()
	if err != nil {
		return fail("open workflow update transaction", err)
	}
	defer tx.Rollback()

	if newLabel != nil || newEnabled != nil {
		var labelToSet string
		var enabledToSet bool
		if err := tx.QueryRowx(`SELECT label, enabled FROM workflow WHERE id=$1`, workflowID).Scan(&labelToSet, &enabledToSet); err != nil {
			return fail("find existing workflow", err)
		}

		if newLabel != nil {
			labelToSet = *newLabel
		}
		if newEnabled != nil {
			enabledToSet = *newEnabled
		}

		if _, err := tx.Exec(`
			UPDATE workflow
			WHERE id=$1
			SET (updated_at, label, enabled) = (current_timestamp, $2, $3)
			`,
			workflowID, labelToSet, enabledToSet); err != nil {
			return fail("update workflow row", err)
		}
	}

	if newCriteria != nil {
		var criteriaIDs []uuid.UUID
		if newCriteria != nil {
			criteriaIDs := make([]uuid.UUID, len(*newCriteria))
			for i, v := range *newCriteria {
				criteriaIDs[i] = v.ID
			}
		}

		// Insert workflow criteria, updating existing criteria
		tx.NamedExec(`
			INSERT INTO workflow_criteria(id, created_at, updated_at, match_key, match_type, match_combine_type, match_value, workflow_id)
			VALUES(:id, current_timestamp, current_timestamp, :match_key, :match_type, :match_combine_type, :match_value, '`+workflowID.String()+`')
			ON CONFLICT DO UPDATE
				SET (updated_at, match_key, match_type, match_combine_type, match_value) =
					(current_timestamp, EXCLUDED.match_key, EXCLUDED.match_type, EXCLUDED.match_combine_type, EXCLUDED.match_value)
		`, *newCriteria)

		// Drop workflow criteria rows which are no longer referenced
		// by this workflow
		if err := execDbIn(tx, `--sql
			DELETE FROM workflow_criteria wc
			WHERE wc.workflow_id=`+workflowID.String()+`
				AND wc.id NOT IN (?)
		`, criteriaIDs); err != nil {
			return fail("bind criteria SQL", err)
		}
	}

	// Drop all workflow targets join table entries and re-create them
	if newTargetIDs != nil {
		if _, err := tx.NamedExec(`DELETE FROM workflow_transcode_targets WHERE workflow_id=$1`, workflowID); err != nil {
			return fail("delete workflow transcode target assocs", err)
		}
		if _, err := tx.NamedExec(`
			INSERT INTO workflow_transcode_targets(id, workflow_id, transcode_target_id)
			VALUES(:id, :workflow_id, :target_id)
			`, workflow.BuildWorkflowTargetAssocs(workflowID, *newTargetIDs),
		); err != nil {
			return fail("create workflow target associations", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fail("commit workflow update transaction", err)
	}
	return nil, nil
}

func (orchestrator *storeOrchestrator) GetWorkflow(id uuid.UUID) *workflow.Workflow {
	return orchestrator.WorkflowStore.Get(orchestrator.db.GetSqlxDb(), id)
}

func (orchestrator *storeOrchestrator) GetAllWorkflows() []*workflow.Workflow {
	return orchestrator.WorkflowStore.GetAll(orchestrator.db.GetSqlxDb())
}

func (orchestrator *storeOrchestrator) DeleteWorkflow(id uuid.UUID) {
	orchestrator.WorkflowStore.Delete(orchestrator.db.GetSqlxDb(), id)
}

// Transcodes

func (orchestrator *storeOrchestrator) SaveTranscode(transcode *transcode.TranscodeTask) error {
	return orchestrator.TranscodeStore.SaveTranscode(orchestrator.db.GetSqlxDb(), transcode)
}
func (orchestrator *storeOrchestrator) GetTranscode(id uuid.UUID) *transcode.TranscodeTask {
	return orchestrator.TranscodeStore.Get(orchestrator.db.GetSqlxDb(), id)
}
func (orchestrator *storeOrchestrator) GetAllTranscodes() ([]*transcode.TranscodeTask, error) {
	return orchestrator.TranscodeStore.GetAll(orchestrator.db.GetSqlxDb())
}
func (orchestrator *storeOrchestrator) GetTranscodesForMedia(mediaId uuid.UUID) ([]*transcode.TranscodeTask, error) {
	return orchestrator.TranscodeStore.GetForMedia(orchestrator.db.GetSqlxDb(), mediaId)
}

// Targets

func (orchestrator *storeOrchestrator) SaveTarget(target *ffmpeg.Target) error {
	return orchestrator.TargetStore.Save(orchestrator.db.GetSqlxDb(), target)
}

func (orchestrator *storeOrchestrator) GetTarget(id uuid.UUID) *ffmpeg.Target {
	return orchestrator.TargetStore.Get(orchestrator.db.GetSqlxDb(), id)
}

func (orchestrator *storeOrchestrator) GetAllTargets() []*ffmpeg.Target {
	return orchestrator.TargetStore.GetAll(orchestrator.db.GetSqlxDb())
}

func (orchestrator *storeOrchestrator) GetManyTargets(ids ...uuid.UUID) []*ffmpeg.Target {
	return orchestrator.TargetStore.GetMany(orchestrator.db.GetSqlxDb(), ids...)
}

func (orchestrator *storeOrchestrator) DeleteTarget(id uuid.UUID) {
	orchestrator.TargetStore.Delete(orchestrator.db.GetSqlxDb(), id)
}

func execDbIn(db *sqlx.Tx, query string, arg any) error {
	if q, a, e := sqlx.In(query, arg); e == nil {
		if _, err := db.Exec(db.Rebind(q), a); err != nil {
			return err
		}
	} else {
		return e
	}

	return nil
}
