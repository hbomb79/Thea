package media

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type (
	dbOrTx interface {
		QueryRowx(query string, args ...interface{}) *sqlx.Row
	}
	// Model contains the union of properties that we expect all store-able information
	// to contain. This is typically basic information about the container.
	Model struct {
		ID        uuid.UUID
		TmdbId    string    `db:"tmdb_id"`
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
		Title     string
	}

	// Watchable represents the union of properties that we expect to see
	// populated on all watchable media (movie/episode). Media containers,
	// such as a series/season are not required to contain this information
	Watchable struct {
		MediaResolution
		SourcePath string `db:"source_path"`
		Adult      bool   `db:"adult"`
	}

	MediaResolution struct {
		Width  int
		Height int
	}

	// Season represents the information Thea stores about a season
	// of episodes itself. A season 'has many' episodes.
	// Additionally, a series is related to many seasons.
	Season struct {
		Model
		SeasonNumber int       `db:"season_number"`
		Episodes     []Episode `db:"-"`
		SeriesID     uuid.UUID `db:"series_id"`
	}

	// Series represents the information Thea stores about a series. A one-to-many
	// relationship exists between series and seasons, although the seasons themselves
	// are not contained within this model.
	Series struct {
		Model
		Seasons []Season `db:"-"`
	}

	// Episode contains all the information unique to an episode, combined
	// with the 'Common' struct.
	Episode struct {
		Model
		Watchable
		SeasonID      uuid.UUID `db:"season_id"`
		EpisodeNumber int       `db:"episode_number"`
	}

	Movie struct {
		Model
		Watchable
	}

	Store struct{}
)

const (
	IDCol     = "id"
	TmdbIDCol = "tmdb_id"

	MediaTable  = "media"
	SeriesTable = "series"
	SeasonTable = "season"
)

// SaveMovie upserts the provided Movie model to the database. Existing models
// to update are found using the 'TmdbId' as this is expected to be a stable
// identifier.
//
// NOTE: the ID of the media may be UPDATED to match existing DB entry (if any)
func (store *Store) SaveMovie(db *sqlx.DB, movie *Movie) error {
	var updatedMovie Movie
	if err := db.QueryRowx(`
		INSERT INTO media(id, type, tmdb_id, title, adult, source_path, created_at, updated_at)
		VALUES($1, $2, $3, $4, $5, $6, current_timestamp, current_timestamp)
		ON CONFLICT(tmdb_id) DO UPDATE
			SET (updated_at, title, adult, source_path, type) = (current_timestamp, EXCLUDED.title, EXCLUDED.adult, EXCLUDED.source_path, 'movie')
		RETURNING id, tmdb_id, title, adult, source_path, created_at, updated_at;
	`, movie.ID, "movie", movie.TmdbId, movie.Title, movie.Adult, movie.SourcePath).StructScan(&updatedMovie); err != nil {
		return err
	}

	// Update provided model to ensure ID and FK are accurate (as updating
	// an existing model doesn't change these as they're immutable)
	movie.ID = updatedMovie.ID
	return nil
}

// SaveSeries upserts the provided Series model to the database. Existing models
// to update are found using the 'TmdbID' as this is expected to be a stable
// identifier.
//
// NOTE: the ID of the media may be UPDATED to match existing DB entry (if any)
func (store *Store) SaveSeries(db dbOrTx, series *Series) error {
	var updatedSeries Series
	if err := db.QueryRowx(`
		INSERT INTO series(id, tmdb_id, title, created_at, updated_at)
		VALUES($1, $2, $3, current_timestamp, current_timestamp)
		ON CONFLICT(tmdb_id) DO UPDATE
			SET (title, updated_at) = (EXCLUDED.title, current_timestamp)
		RETURNING *
	`, series.ID, series.TmdbId, series.Title).StructScan(&updatedSeries); err != nil {
		return err
	}

	// Update provided model to ensure ID and FK are accurate (as updating
	// an existing model doesn't change these as they're immutable)
	series.ID = updatedSeries.ID
	return nil
}

// SaveSeason upserts the provided Season model to the database. Existing models
// to update are found using the 'TmdbID' as this is expected to be a stable
// identifier.
//
// NOTE: the PK and FK ID's of the media may be UPDATED to match existing DB entry (if any)
func (store *Store) SaveSeason(db dbOrTx, season *Season) error {
	var updatedSeason Season
	if err := db.QueryRowx(`
		INSERT INTO season(id, tmdb_id, season_number, title, series_id, created_at, updated_at)
		VALUES($1, $2, $3, $4, $5, current_timestamp, current_timestamp)
		ON CONFLICT(tmdb_id) DO UPDATE
			SET (season_number, title, series_id, updated_at) = (EXCLUDED.season_number, EXCLUDED.title, EXCLUDED.series_id, current_timestamp)
		RETURNING *
	`, season.ID, season.TmdbId, season.SeasonNumber, season.Title, season.SeriesID).StructScan(&updatedSeason); err != nil {
		return err
	}

	// Update provided model to ensure ID and FK are accurate (as updating
	// an existing model doesn't change these as they're immutable)
	season.ID = updatedSeason.ID
	season.SeriesID = updatedSeason.SeriesID
	return nil
}

// saveEpisode transactionally upserts the episode and it's season
// and series. Existing models are found using the models 'TmdbID'
// as this is expected to be a stable identifier.
//
// NOTE: the PK and FK ID's of the media may be UPDATED to match existing DB entry (if any)
func (store *Store) SaveEpisode(db dbOrTx, episode *Episode) error {
	var updatedEpisode Episode
	if err := db.QueryRowx(`
		INSERT INTO media(id, type, tmdb_id, episode_number, title, source_path, season_id, adult, created_at, updated_at)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, current_timestamp, current_timestamp)
		ON CONFLICT(tmdb_id) DO UPDATE
			SET (episode_number, title, source_path, season_id, updated_at, type, adult) =
				(EXCLUDED.episode_number, EXCLUDED.title, EXCLUDED.source_path, EXCLUDED.season_id, current_timestamp, 'episode', EXCLUDED.adult)
		RETURNING id, tmdb_id, episode_number, title, source_path, season_id, adult, created_at, updated_at;
	`, episode.ID, "episode", episode.TmdbId, episode.EpisodeNumber, episode.Title, episode.SourcePath, episode.SeasonID, episode.Adult).StructScan(&updatedEpisode); err != nil {
		return err
	}

	// Update provided model to ensure ID and FK are accurate (as updating
	// an existing model doesn't change these as they're immutable)
	episode.ID = updatedEpisode.ID
	episode.SeasonID = updatedEpisode.SeasonID
	return nil
}

// GetMedia is a convinience method for requesting either a Movie
// or an Episode. The ID provided is used to lookup both, and whichever
// query is successful is used to populate a media Container.
func (store *Store) GetMedia(db *sqlx.DB, mediaID uuid.UUID) *Container {
	if movie, err := store.GetMovie(db, mediaID); err != nil {
		if episode, err := store.GetEpisode(db, mediaID); err != nil {
			return nil
		} else {
			return &Container{
				Type:    EPISODE,
				Episode: episode,
				Movie:   nil,
			}
		}
	} else {
		return &Container{
			Type:    MOVIE,
			Movie:   movie,
			Episode: nil,
		}
	}
}

// GetMovie searches for an existing movie with the Thea PK ID provided.
func (store *Store) GetMovie(db *sqlx.DB, movieID uuid.UUID) (*Movie, error) {
	return queryRow[Movie](db, MediaTable, IDCol, movieID)
}

// GetMovieWithTmdbId searches for an existing movie with the TMDB unique ID provided.
func (store *Store) GetMovieWithTmdbId(db *sqlx.DB, tmdbID string) (*Movie, error) {
	return queryRow[Movie](db, MediaTable, TmdbIDCol, tmdbID)
}

// GetSeries searches for an existing series with the Thea PK ID provided.
func (store *Store) GetSeries(db *sqlx.DB, seriesID uuid.UUID) (*Series, error) {
	return queryRow[Series](db, SeriesTable, IDCol, seriesID)
}

// GetSeriesWithTmdbId searches for an existing series with the TMDB unique ID provided.
func (store *Store) GetSeriesWithTmdbId(db *sqlx.DB, tmdbID string) (*Series, error) {
	return queryRow[Series](db, SeriesTable, TmdbIDCol, tmdbID)
}

// GetSeason searches for an existing season with the Thea PK ID provided.
func (store *Store) GetSeason(db *sqlx.DB, seasonID uuid.UUID) (*Season, error) {
	return queryRow[Season](db, SeasonTable, IDCol, seasonID)
}

// GetSeasonWithTmdbId searches for an existing season with the TMDB unique ID provided.
func (store *Store) GetSeasonWithTmdbId(db *sqlx.DB, tmdbID string) (*Season, error) {
	return queryRow[Season](db, SeasonTable, TmdbIDCol, tmdbID)
}

// GetEpisode searches for an existing episode with the Thea PK ID provided.
func (store *Store) GetEpisode(db *sqlx.DB, episodeID uuid.UUID) (*Episode, error) {
	return queryRow[Episode](db, MediaTable, IDCol, episodeID)
}

// GetEpisodeWithTmdbId searches for an existing episode with the TMDB unique ID provided.
func (store *Store) GetEpisodeWithTmdbId(db *sqlx.DB, tmdbID string) (*Episode, error) {
	return queryRow[Episode](db, MediaTable, TmdbIDCol, tmdbID)
}

// GetAllSourcePaths returns all the source paths related
// to media that is currently known to Thea by polling the database.
func (store *Store) GetAllSourcePaths(db *sqlx.DB) ([]string, error) {
	var paths []string
	if err := db.Select(&paths, `SELECT source_path FROM media`); err != nil {
		return nil, err
	}

	return paths, nil
}

func queryRow[T any](db *sqlx.DB, table string, col string, val any) (*T, error) {
	v := new(T)
	if err := db.Get(&v, fmt.Sprintf(`SELECT * FROM %s WHERE %s=$1;`, table, col), val); err != nil {
		return nil, err
	}

	return v, nil
}
