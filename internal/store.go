package internal

import (
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"github.com/hbomb79/Thea/internal/event"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/internal/transcode"
	"github.com/hbomb79/Thea/internal/user"
	"github.com/hbomb79/Thea/internal/workflow"
	"github.com/hbomb79/Thea/internal/workflow/match"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

const (
	PgFkConstraintViolationCode = "23503"
)

var (
	ErrDatabaseNotConnected    = errors.New("cannot construct thea data store with a disconnected db")
	ErrWorkflowTargetIDMissing = errors.New("one or more of the targets provided cannot be found")
)

// storeOrchestrator is responsible for managing all of Thea's resources,
// especially highly-relational data. You can think of all
// the data stores below this layer being 'dumb', and this store
// linking them together and providing the database instance
//
// If consumers need to be able to access data stores directly, they're
// welcome to do so - however caution should be taken as stores have no
// obligation to take care of relational data (which is the orchestrator's job).
type storeOrchestrator struct {
	db             database.Manager
	ev             event.EventDispatcher
	mediaStore     *media.Store
	transcodeStore *transcode.Store
	workflowStore  *workflow.Store
	targetStore    *ffmpeg.Store
	userStore      *user.Store
}

func newStoreOrchestrator(db database.Manager, eventBus event.EventDispatcher) (*storeOrchestrator, error) {
	if db.GetSqlxDB() == nil {
		return nil, ErrDatabaseNotConnected
	}

	return &storeOrchestrator{
		db:             db,
		ev:             eventBus,
		mediaStore:     &media.Store{},
		transcodeStore: &transcode.Store{},
		workflowStore:  &workflow.Store{},
		targetStore:    &ffmpeg.Store{},
		userStore:      user.NewStore(),
	}, nil
}

func (orchestrator *storeOrchestrator) GetMedia(mediaID uuid.UUID) *media.Container {
	return orchestrator.mediaStore.GetMedia(orchestrator.db.GetSqlxDB(), mediaID)
}

func (orchestrator *storeOrchestrator) GetMovie(movieID uuid.UUID) (*media.Movie, error) {
	var movie *media.Movie
	if err := orchestrator.db.WrapTx(func(tx *sqlx.Tx) error {
		m, err := orchestrator.mediaStore.GetMovie(tx, movieID)
		if err != nil {
			return err
		}

		genres, err := orchestrator.mediaStore.GetGenresForMovie(tx, movieID)
		if err != nil {
			return err
		}

		m.Genres = genres
		movie = m

		return nil
	}); err != nil {
		return nil, err
	}

	return movie, nil
}

func (orchestrator *storeOrchestrator) GetEpisode(episodeID uuid.UUID) (*media.Episode, error) {
	return orchestrator.mediaStore.GetEpisode(orchestrator.db.GetSqlxDB(), episodeID)
}

func (orchestrator *storeOrchestrator) GetEpisodeWithTmdbID(tmdbID string) (*media.Episode, error) {
	return orchestrator.mediaStore.GetEpisodeWithTmdbID(orchestrator.db.GetSqlxDB(), tmdbID)
}

func (orchestrator *storeOrchestrator) GetSeason(seasonID uuid.UUID) (*media.Season, error) {
	return orchestrator.mediaStore.GetSeason(orchestrator.db.GetSqlxDB(), seasonID)
}

func (orchestrator *storeOrchestrator) GetSeasonWithTmdbID(tmdbID string) (*media.Season, error) {
	return orchestrator.mediaStore.GetSeasonWithTmdbID(orchestrator.db.GetSqlxDB(), tmdbID)
}

func (orchestrator *storeOrchestrator) GetSeries(seriesID uuid.UUID) (*media.Series, error) {
	return orchestrator.mediaStore.GetSeries(orchestrator.db.GetSqlxDB(), seriesID)
}

func (orchestrator *storeOrchestrator) GetSeriesWithTmdbID(tmdbID string) (*media.Series, error) {
	return orchestrator.mediaStore.GetSeriesWithTmdbID(orchestrator.db.GetSqlxDB(), tmdbID)
}

func (orchestrator *storeOrchestrator) GetAllMediaSourcePaths() ([]string, error) {
	return orchestrator.mediaStore.GetAllSourcePaths(orchestrator.db.GetSqlxDB())
}

// SaveMovie transactionally saves the given Movie model and it's genre
// information to the database.
func (orchestrator *storeOrchestrator) SaveMovie(movie *media.Movie) error {
	return orchestrator.db.WrapTx(func(tx *sqlx.Tx) error {
		if err := orchestrator.mediaStore.SaveMovie(tx, movie); err != nil {
			return err
		}

		log.Verbosef("Saving genres %v\n", movie.Genres)
		genres, err := orchestrator.mediaStore.SaveGenres(tx, movie.Genres)
		if err != nil {
			return err
		}

		log.Verbosef("Saving genres assocations %v for movie_id=%s\n", genres, movie.ID)
		return orchestrator.mediaStore.SaveMovieGenreAssociations(tx, movie.ID, genres)
	})
}

// SaveEpisode transactionally saves the episode provided, as well as the season and series
// it's associatted with. Existing models are updating ON CONFLICT with the TmdbID unique
// identifier. The PK's and relational FK's of the models will automatically be
// set during saving.
//
// Note: If the season/series are not provided, and the FK-constraint of the episode cannot
// be fulfilled because of this, then the save will fail. It is recommended to supply all parameters.
func (orchestrator *storeOrchestrator) SaveEpisode(episode *media.Episode, season *media.Season, series *media.Series) error {
	// Store old PK/FKs so we can rollback on transaction failure
	episodeID := episode.ID
	seasonID := season.ID
	seriesID := series.ID
	episodeFk := episode.SeasonID
	seasonFk := season.SeriesID

	if err := orchestrator.db.WrapTx(func(tx *sqlx.Tx) error {
		log.Verbosef("Saving series %#v\n", series)
		if err := orchestrator.mediaStore.SaveSeries(tx, series); err != nil {
			return err
		}

		log.Verbosef("Saving genres %v\n", series.Genres)
		genres, err := orchestrator.mediaStore.SaveGenres(tx, series.Genres)
		if err != nil {
			return err
		}

		log.Verbosef("Saving genres associations %v for series_id=%s\n", genres, series.ID)
		if err := orchestrator.mediaStore.SaveSeriesGenreAssociations(tx, series.ID, genres); err != nil {
			return err
		}

		log.Verbosef("Saving season %#v with series_id=%s\n", season, series.ID)
		season.SeriesID = series.ID
		if err := orchestrator.mediaStore.SaveSeason(tx, season); err != nil {
			return err
		}

		log.Verbosef("Saving episode %#v with season_id=%s\n", episode, seasonID)
		episode.SeasonID = season.ID
		return orchestrator.mediaStore.SaveEpisode(tx, episode)
	}); err != nil {
		log.Warnf(
			"Episode save failed, rolling back model keys (epID=%s, epFK=%s, seasonID=%s, seasonFK=%s, seriesID=%s)",
			episodeID, episodeFk, seasonID, seasonFk, seriesID,
		)

		episode.ID = episodeID
		season.ID = seasonID
		series.ID = seriesID

		episode.SeasonID = episodeFk
		season.SeriesID = seasonFk
		return err
	}

	return nil
}

func (orchestrator *storeOrchestrator) ListMovie() ([]*media.Movie, error) {
	return orchestrator.mediaStore.ListMovie(orchestrator.db.GetSqlxDB())
}

func (orchestrator *storeOrchestrator) ListSeries() ([]*media.Series, error) {
	return orchestrator.mediaStore.ListSeries(orchestrator.db.GetSqlxDB())
}

func (orchestrator *storeOrchestrator) ListGenres() ([]*media.Genre, error) {
	return orchestrator.mediaStore.ListGenres(orchestrator.db.GetSqlxDB())
}

func (orchestrator *storeOrchestrator) ListMedia(
	includeTypes []media.MediaListType,
	titleFilter string,
	includeGenres []int,
	orderBy []media.MediaListOrderBy,
	offset int,
	limit int,
) ([]*media.MediaListResult, error) {
	return orchestrator.mediaStore.ListMedia(orchestrator.db.GetSqlxDB(), titleFilter, includeTypes, includeGenres, orderBy, offset, limit)
}

func (orchestrator *storeOrchestrator) CountSeasonsInSeries(seriesIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	return orchestrator.mediaStore.CountSeasonsInSeries(orchestrator.db.GetSqlxDB(), seriesIDs)
}

func (orchestrator *storeOrchestrator) GetEpisodesForSeries(seriesID uuid.UUID) ([]*media.Episode, error) {
	episodes, err := orchestrator.mediaStore.GetEpisodesForSeries(orchestrator.db.GetSqlxDB(), []uuid.UUID{seriesID})
	if err != nil {
		return nil, err
	}

	if eps, ok := episodes[seriesID]; ok {
		return eps, nil
	}

	return []*media.Episode{}, nil
}

func (orchestrator *storeOrchestrator) GetEpisodesForSeason(seasonID uuid.UUID) ([]*media.Episode, error) {
	episodes, err := orchestrator.mediaStore.GetEpisodesForSeasons(orchestrator.db.GetSqlxDB(), []uuid.UUID{seasonID})
	if err != nil {
		return nil, err
	}

	if eps, ok := episodes[seasonID]; ok {
		return eps, nil
	}

	return []*media.Episode{}, nil
}

func (orchestrator *storeOrchestrator) GetInflatedSeries(seriesID uuid.UUID) (*media.InflatedSeries, error) {
	wrap := func(err error) error {
		return fmt.Errorf("failed to fetch inflated series: %w", err)
	}

	var inflated *media.InflatedSeries
	if err := orchestrator.db.WrapTx(func(tx *sqlx.Tx) error {
		// Fetch the series
		series, err := orchestrator.mediaStore.GetSeries(tx, seriesID)
		if err != nil {
			return err
		}

		genres, err := orchestrator.mediaStore.GetGenresForSeries(tx, seriesID)
		if err != nil {
			return err
		}
		series.Genres = genres

		// Fetch all seasons for series
		seasons, err := orchestrator.mediaStore.GetSeasonsForSeries(tx, seriesID)
		if err != nil {
			return err
		}

		seasonIDs := make([]uuid.UUID, len(seasons))
		for k, v := range seasons {
			seasonIDs[k] = v.ID
		}

		// Fetch all episodes for all series
		episodes, err := orchestrator.mediaStore.GetEpisodesForSeasons(tx, seasonIDs)
		if err != nil {
			return err
		}

		// Package the results in to the InflatedSeries
		inflatedSeasons := make([]*media.InflatedSeason, len(seasons))
		for k, v := range seasons {
			eps := episodes[v.ID]
			inflatedSeasons[k] = &media.InflatedSeason{Season: v, Episodes: eps}
		}

		inflated = &media.InflatedSeries{
			Series:  series,
			Seasons: inflatedSeasons,
		}
		return nil
	}); err != nil {
		return nil, wrap(err)
	}

	return inflated, nil
}

// Transactionally lists all series in the DB, and then submits a second query to fetch the number of seasons
// associated with the series we found. This information is then packaged inside the SeriesStub struct.
func (orchestrator *storeOrchestrator) ListSeriesStubs() ([]*media.SeriesStub, error) {
	wrap := func(err error) error {
		return fmt.Errorf("failed to list series stubs: %w", err)
	}

	var inflated []*media.SeriesStub
	if err := orchestrator.db.WrapTx(func(tx *sqlx.Tx) error {
		series, err := orchestrator.mediaStore.ListSeries(tx)
		if err != nil {
			return err
		}

		seriesIDs := make([]uuid.UUID, len(series))
		for k, v := range series {
			seriesIDs[k] = v.ID
		}

		seasonCounts, err := orchestrator.mediaStore.CountSeasonsInSeries(tx, seriesIDs)
		if err != nil {
			return err
		}

		// TODO: grab ratings, cast, etc etc... once we actually store this information xD

		inflated = make([]*media.SeriesStub, len(seriesIDs))
		for k, v := range series {
			seasonCount := -1
			if count, ok := seasonCounts[v.ID]; ok {
				seasonCount = count
			}

			inflated[k] = &media.SeriesStub{Series: v, SeasonCount: seasonCount}
		}

		return nil
	}); err != nil {
		return nil, wrap(err)
	}

	return inflated, nil
}

// ** Media deletion is a little bit tricky, but the general shape is:
// 1. Fetch completed transcodes for the media (or it's children, if we're deleting a series/season).
// 2. Delete the above completed transcodes from the database, *and* the filesystem.
// 3. Now, delete the media itself from the database. This will FAIL if a new transcode for the media
//	  was inserted between this step and the last due to the use of ON DELETE RESTRICT on the FK
// 4. Finally, cancel all on-going transcodes (via the event bus) for the relevant medias now that we've dealt with the
//    database entries.

func (orchestrator *storeOrchestrator) DeleteMovie(movieID uuid.UUID) error {
	if err := orchestrator.DeleteTranscodesForMedia(movieID); err != nil {
		return fmt.Errorf("failed to delete existing transcodes: %w", err)
	}
	if err := orchestrator.mediaStore.DeleteMovie(orchestrator.db.GetSqlxDB(), movieID); err != nil {
		return err
	}

	orchestrator.ev.Dispatch(event.DeleteMediaEvent, movieID)
	return nil
}

func (orchestrator *storeOrchestrator) DeleteSeries(seriesID uuid.UUID) error {
	episodes, err := orchestrator.GetEpisodesForSeries(seriesID)
	if err != nil {
		return err
	}

	episodeIDs := make([]uuid.UUID, len(episodes))
	for k, v := range episodes {
		episodeIDs[k] = v.ID
	}

	if err := orchestrator.DeleteTranscodesForMedias(episodeIDs); err != nil {
		return fmt.Errorf("failed to delete existing transcodes: %w", err)
	}
	if err := orchestrator.mediaStore.DeleteSeries(orchestrator.db.GetSqlxDB(), seriesID); err != nil {
		return err
	}

	for _, id := range episodeIDs {
		orchestrator.ev.Dispatch(event.DeleteMediaEvent, id)
	}

	return nil
}

func (orchestrator *storeOrchestrator) DeleteSeason(seasonID uuid.UUID) error {
	episodes, err := orchestrator.GetEpisodesForSeason(seasonID)
	if err != nil {
		return err
	}

	episodeIDs := make([]uuid.UUID, len(episodes))
	for k, v := range episodes {
		episodeIDs[k] = v.ID
	}

	if err := orchestrator.DeleteTranscodesForMedias(episodeIDs); err != nil {
		return fmt.Errorf("failed to delete existing transcodes: %w", err)
	}
	if err := orchestrator.mediaStore.DeleteSeason(orchestrator.db.GetSqlxDB(), seasonID); err != nil {
		return err
	}

	for _, id := range episodeIDs {
		orchestrator.ev.Dispatch(event.DeleteMediaEvent, id)
	}

	return nil
}

func (orchestrator *storeOrchestrator) DeleteEpisode(episodeID uuid.UUID) error {
	if err := orchestrator.DeleteTranscodesForMedia(episodeID); err != nil {
		return fmt.Errorf("failed to delete existing transcodes: %w", err)
	}
	if err := orchestrator.mediaStore.DeleteEpisode(orchestrator.db.GetSqlxDB(), episodeID); err != nil {
		return err
	}

	orchestrator.ev.Dispatch(event.DeleteMediaEvent, episodeID)
	return nil
}

// Workflows

// CreateWorkflow uses the information provided to construct and save a new workflow
// in a single DB transaction.
//
// Error will be returned if any of the target IDs provided do not refer to existing Target
// DB entries, or if the workflow infringes on any uniqueness constraints (label).
func (orchestrator *storeOrchestrator) CreateWorkflow(workflowID uuid.UUID, label string, criteria []match.Criteria, targetIDs []uuid.UUID, enabled bool) (*workflow.Workflow, error) {
	db := orchestrator.db.GetSqlxDB()
	if err := orchestrator.workflowStore.Create(db, workflowID, label, enabled, targetIDs, criteria); err != nil {
		return nil, err
	}

	return orchestrator.workflowStore.Get(db, workflowID), nil
}

// UpdateWorkflow transactionally updates an existing Workflow model
// using the optional parameters provided. If a param is `nil` then the
// corresponding value in the model is NOT changed.
func (orchestrator *storeOrchestrator) UpdateWorkflow(workflowID uuid.UUID, newLabel *string, newCriteria *[]match.Criteria, newTargetIDs *[]uuid.UUID, newEnabled *bool) (*workflow.Workflow, error) {
	fail := func(desc string, err error) error {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			if pqErr.Code == PgFkConstraintViolationCode && pqErr.Table == "workflow_transcode_targets" {
				log.Debugf("DB query failure; apparent target ID FK violation %#v\n", err)
				return ErrWorkflowTargetIDMissing
			}
		}

		log.Errorf("Unexpected query failure: %v\n", err)
		return fmt.Errorf("failed to %s due to unexpected query error: %w", desc, err)
	}

	err := orchestrator.db.WrapTx(func(tx *sqlx.Tx) error {
		if newLabel != nil || newEnabled != nil {
			if err := orchestrator.workflowStore.UpdateWorkflowTx(tx, workflowID, newLabel, newEnabled); err != nil {
				return fail("update workflow row", err)
			}
		}
		if newCriteria != nil {
			if err := orchestrator.workflowStore.UpdateWorkflowCriteriaTx(tx, workflowID, *newCriteria); err != nil {
				return fail("update workflow criteria associations", err)
			}
		}
		if newTargetIDs != nil {
			if err := orchestrator.workflowStore.UpdateWorkflowTargetsTx(tx, workflowID, *newTargetIDs); err != nil {
				return fail("update workflow target associations", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return orchestrator.workflowStore.Get(orchestrator.db.GetSqlxDB(), workflowID), nil
}

func (orchestrator *storeOrchestrator) GetWorkflow(id uuid.UUID) *workflow.Workflow {
	return orchestrator.workflowStore.Get(orchestrator.db.GetSqlxDB(), id)
}

func (orchestrator *storeOrchestrator) GetAllWorkflows() []*workflow.Workflow {
	all := orchestrator.workflowStore.GetAll(orchestrator.db.GetSqlxDB())
	return all
}

func (orchestrator *storeOrchestrator) DeleteWorkflow(id uuid.UUID) {
	orchestrator.workflowStore.Delete(orchestrator.db.GetSqlxDB(), id)
}

// Transcodes

func (orchestrator *storeOrchestrator) SaveTranscode(transcode *transcode.TranscodeTask) error {
	return orchestrator.transcodeStore.SaveTranscode(orchestrator.db.GetSqlxDB(), transcode)
}

func (orchestrator *storeOrchestrator) GetTranscode(id uuid.UUID) *transcode.Transcode {
	return orchestrator.transcodeStore.Get(orchestrator.db.GetSqlxDB(), id)
}

func (orchestrator *storeOrchestrator) GetAllTranscodes() ([]*transcode.Transcode, error) {
	return orchestrator.transcodeStore.GetAll(orchestrator.db.GetSqlxDB())
}

func (orchestrator *storeOrchestrator) GetTranscodesForMedia(mediaID uuid.UUID) ([]*transcode.Transcode, error) {
	return orchestrator.transcodeStore.GetForMedia(orchestrator.db.GetSqlxDB(), mediaID)
}

func (orchestrator *storeOrchestrator) DeleteTranscode(id uuid.UUID) error {
	transcodePath, err := orchestrator.transcodeStore.Delete(orchestrator.db.GetSqlxDB(), id)
	if err != nil {
		return err
	}

	if err := os.Remove(transcodePath); err != nil {
		log.Warnf("Cleanup of transcode at path '%s' failed: %v\n", transcodePath, err)
	}

	return nil
}

func (orchestrator *storeOrchestrator) DeleteTranscodesForMedia(mediaID uuid.UUID) error {
	return orchestrator.DeleteTranscodesForMedias([]uuid.UUID{mediaID})
}

func (orchestrator *storeOrchestrator) DeleteTranscodesForMedias(mediaIDs []uuid.UUID) error {
	paths, err := orchestrator.transcodeStore.DeleteForMedias(orchestrator.db.GetSqlxDB(), mediaIDs)
	if err != nil {
		return err
	}

	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			log.Warnf("Cleanup of transcode at path '%s' failed: %v\n", path, err)
		}
	}

	return nil
}

func (orchestrator *storeOrchestrator) GetForMediaAndTarget(mediaID uuid.UUID, targetID uuid.UUID) (*transcode.Transcode, error) {
	return orchestrator.transcodeStore.GetForMediaAndTarget(orchestrator.db.GetSqlxDB(), mediaID, targetID)
}

// Targets

func (orchestrator *storeOrchestrator) SaveTarget(target *ffmpeg.Target) error {
	return orchestrator.targetStore.Save(orchestrator.db.GetSqlxDB(), target)
}

func (orchestrator *storeOrchestrator) GetTarget(id uuid.UUID) *ffmpeg.Target {
	return orchestrator.targetStore.Get(orchestrator.db.GetSqlxDB(), id)
}

func (orchestrator *storeOrchestrator) GetAllTargets() []*ffmpeg.Target {
	return orchestrator.targetStore.GetAll(orchestrator.db.GetSqlxDB())
}

func (orchestrator *storeOrchestrator) GetManyTargets(ids ...uuid.UUID) []*ffmpeg.Target {
	return orchestrator.targetStore.GetMany(orchestrator.db.GetSqlxDB(), ids...)
}

func (orchestrator *storeOrchestrator) DeleteTarget(id uuid.UUID) {
	orchestrator.targetStore.Delete(orchestrator.db.GetSqlxDB(), id)
}

// User Management

func (orchestrator *storeOrchestrator) GetUserWithUsernameAndPassword(username []byte, password []byte) (*user.User, error) {
	return orchestrator.userStore.GetWithUsernameAndPassword(orchestrator.db.GetSqlxDB(), username, password)
}

func (orchestrator *storeOrchestrator) GetUserWithID(id uuid.UUID) (*user.User, error) {
	return orchestrator.userStore.GetWithID(orchestrator.db.GetSqlxDB(), id)
}

func (orchestrator *storeOrchestrator) CreateUser(username []byte, password []byte, permissions ...string) (*user.User, error) {
	if len(permissions) == 0 {
		return orchestrator.userStore.Create(orchestrator.db.GetSqlxDB(), username, password)
	}

	var outputUser *user.User
	if err := orchestrator.db.WrapTx(func(tx *sqlx.Tx) error {
		user, err := orchestrator.userStore.Create(tx, username, password)
		if err != nil {
			return err
		}

		outputUser = user
		return orchestrator.updateUserPermissionsQuery(tx, user.ID, permissions)
	}); err != nil {
		return nil, err
	}

	return outputUser, nil
}

func (orchestrator *storeOrchestrator) ListUsers() ([]*user.User, error) {
	return orchestrator.userStore.List(orchestrator.db.GetSqlxDB())
}

func (orchestrator *storeOrchestrator) RecordUserLogin(userID uuid.UUID) error {
	return orchestrator.userStore.RecordLogin(orchestrator.db.GetSqlxDB(), userID)
}

func (orchestrator *storeOrchestrator) RecordUserRefresh(userID uuid.UUID) error {
	return orchestrator.userStore.RecordRefresh(orchestrator.db.GetSqlxDB(), userID)
}

func (orchestrator *storeOrchestrator) UpdateUserPermissions(userID uuid.UUID, newPermissions []string) error {
	return orchestrator.db.WrapTx(func(tx *sqlx.Tx) error { return orchestrator.updateUserPermissionsQuery(tx, userID, newPermissions) })
}

func (orchestrator *storeOrchestrator) updateUserPermissionsQuery(tx *sqlx.Tx, userID uuid.UUID, newPermissions []string) error {
	if err := orchestrator.userStore.DropUserPermissions(tx, userID); err != nil {
		return err
	}

	if err := orchestrator.userStore.RecordUpdate(tx, userID); err != nil {
		return err
	}

	if len(newPermissions) > 0 {
		perms, err := orchestrator.userStore.GetPermissionsByLabel(tx, newPermissions)
		if err != nil {
			return err
		}

		if len(perms) != len(newPermissions) {
			return errors.New("permissions provided are invalid")
		}

		if err := orchestrator.userStore.InsertUserPermissions(tx, userID, perms); err != nil {
			return err
		}
	}

	return nil
}

func (orchestrator *storeOrchestrator) anyOutstandingPermissions(permissions ...string) (bool, error) {
	query, args, err := sqlx.In(`SELECT label FROM permissions WHERE label NOT IN(?)`, permissions)
	if err != nil {
		return false, err
	}

	var labels []string
	db := orchestrator.db.GetSqlxDB()
	if err := db.Select(&labels, db.Rebind(query), args...); err != nil {
		return false, err
	}

	if len(labels) > 0 {
		log.Warnf("Found outstanding permissions: %v\n", labels)
		return true, nil
	}

	return false, nil
}

func (orchestrator *storeOrchestrator) createPermissions(permissions ...string) error {
	type p struct {
		ID    uuid.UUID `db:"id"`
		Label string    `db:"label"`
	}

	perms := make([]p, len(permissions))
	for k, v := range permissions {
		perms[k] = p{uuid.New(), v}
	}

	_, err := orchestrator.db.GetSqlxDB().NamedExec(
		`INSERT INTO permissions(id, label) VALUES (:id, :label) ON CONFLICT(label) DO NOTHING`,
		perms,
	)

	return err
}
