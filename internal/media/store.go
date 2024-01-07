package media

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type (
	// Model contains the union of properties that we expect all store-able information
	// to contain. This is typically basic information about the container.
	Model struct {
		ID        uuid.UUID
		TmdbID    string    `db:"tmdb_id"`
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
		Title     string
	}

	// Media represents the form of both movies and episodes inside the database. It is only after checking the
	// type of the Media row that we can determine whether the row represents a movie or an episode.
	media struct {
		Model
		Watchable
		Type          string     `db:"type"`
		EpisodeNumber *int       `db:"episode_number"` // Nullable
		SeasonID      *uuid.UUID `db:"season_id"`      // Nullable
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
		SeriesID     uuid.UUID `db:"series_id"`
	}

	Genre struct {
		Id    int    `db:"id" json:"id"`
		Label string `db:"label" json:"label"`
	}

	// Series represents the information Thea stores about a series. A one-to-many
	// relationship exists between series and seasons, although the seasons themselves
	// are not contained within this model.
	Series struct {
		Model
		Genres []*Genre
	}

	// SeriesStub is used to package information about a series which doesn't map one-to-one with
	// it's databse representation.
	//
	// NB: this struct does not represent a table which exists in the DB, it is purely used to package
	// query results that arise from joining multiple tables together.
	SeriesStub struct {
		*Series
		SeasonCount int
		//TODO: ratings (anything else?)
	}

	// InflatedSeries follows a similar principal to SeriesStub, in that is represents a Series *along with* other information
	// which has been retrived using table joins/additional queries. It's a representation of the basic Series entity after
	// being 'inflated' with all the information we might reasonably want to bundle with it (such as seasons and episode [stubs]).
	InflatedSeries struct {
		*Series
		Seasons []*InflatedSeason
		//TODO: cast members, ratings, etc
	}

	InflatedSeason struct {
		*Season
		Episodes []*Episode
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
		Genres []*Genre
	}

	MediaListResult struct {
		Series *SeriesStub
		Movie  *Movie
	}
	MediaListType        string
	MediaListOrderColumn string
	MediaListOrderBy     struct {
		Column MediaListOrderColumn
		// Descending controls the ordering for this column:
		//  - true -> DESC order
		//  - false -> ASC order
		Descending bool
	}

	jsonColumn[T any] struct {
		val *T
	}

	Store struct{}
)

var (
	storeLogger = logger.Get("MediaStore")
)

const (
	IDCol     = "id"
	TmdbIDCol = "tmdb_id"

	MediaTable  = "media"
	SeriesTable = "series"
	SeasonTable = "season"

	MediaMovieClause   = "AND type='movie'"
	MediaEpisodeClause = "AND type='episode'"
)

type mediaType int

const (
	episodeType mediaType = iota
	movieType
)

func (j *jsonColumn[T]) Scan(src any) error {
	if src == nil {
		j.val = nil
		return nil
	}

	j.val = new(T)
	return json.Unmarshal(src.([]byte), j.val)
}

func (j *jsonColumn[T]) Get() *T {
	return j.val
}

// SaveMovie upserts the provided Movie model to the database. Existing models
// to update are found using the 'TmdbId' as this is expected to be a stable
// identifier.
//
// NOTE: the ID of the media may be UPDATED to match existing DB entry (if any)
func (store *Store) SaveMovie(db database.Queryable, movie *Movie) error {
	var updatedMovie Movie
	if err := db.QueryRowx(`
		INSERT INTO media(id, type, tmdb_id, title, adult, source_path, created_at, updated_at)
		VALUES($1, $2, $3, $4, $5, $6, current_timestamp, current_timestamp)
		ON CONFLICT(tmdb_id, type) DO UPDATE
			SET (updated_at, title, adult, source_path) = (current_timestamp, EXCLUDED.title, EXCLUDED.adult, EXCLUDED.source_path)
		RETURNING id, tmdb_id, title, adult, source_path, created_at, updated_at;
	`, movie.ID, "movie", movie.TmdbID, movie.Title, movie.Adult, movie.SourcePath).StructScan(&updatedMovie); err != nil {
		return err
	}

	// Update provided model to ensure ID and FK are accurate (as updating
	// an existing model doesn't change these as they're immutable)
	movie.ID = updatedMovie.ID
	return nil
}

// SaveMovieGenreAssociations handles only the upserting of the genre associations
// for a given movie model.
//
// NB: This query will FAIL if any of the given genres do not have a row in the genre table
func (store *Store) SaveMovieGenreAssociations(db database.Queryable, movieID uuid.UUID, genres []*Genre) error {
	if len(genres) > 0 {
		type genreAssoc struct {
			ID      uuid.UUID `db:"id"`
			MovieID uuid.UUID `db:"movie_id"`
			GenreID int       `db:"genre_id"`
		}
		genreAssocs := make([]genreAssoc, len(genres))
		for k, v := range genres {
			genreAssocs[k] = genreAssoc{uuid.New(), movieID, v.Id}
		}

		if err := database.InExec(db, `DELETE FROM movie_genres mg WHERE mg.movie_id=$1`, movieID); err != nil {
			return err
		}

		_, err := db.NamedExec(`
			INSERT INTO movie_genres(id, movie_id, genre_id)
			VALUES(:id, :movie_id, :genre_id)
			ON CONFLICT(movie_id, genre_id) DO NOTHING
		`, genreAssocs)

		return err
	}

	_, err := db.Exec(`
		DELETE FROM movie_genres WHERE media_id=$1`, movieID)
	return err
}

// SaveSeries upserts the provided Series model to the database. Existing models
// to update are found using the 'TmdbID' as this is expected to be a stable
// identifier.
//
// NOTE: the ID of the media may be UPDATED to match existing DB entry (if any)
func (store *Store) SaveSeries(db database.Queryable, series *Series) error {
	var updatedSeries Series
	if err := db.QueryRowx(`
		INSERT INTO series(id, tmdb_id, title, created_at, updated_at)
		VALUES($1, $2, $3, current_timestamp, current_timestamp)
		ON CONFLICT(tmdb_id) DO UPDATE
			SET (title, updated_at) = (EXCLUDED.title, current_timestamp)
		RETURNING *
	`, series.ID, series.TmdbID, series.Title).StructScan(&updatedSeries); err != nil {
		return err
	}

	// Update provided model to ensure ID and FK are accurate (as updating
	// an existing model doesn't change these as they're immutable)
	series.ID = updatedSeries.ID
	return nil
}

// SaveMovieGenreAssociations handles only the upserting of the genre associations
// for a given movie model.
//
// NB: This query will FAIL if any of the given genres do not have a row in the genre table
func (store *Store) SaveSeriesGenreAssociations(db database.Queryable, seriesID uuid.UUID, genres []*Genre) error {
	if len(genres) > 0 {
		type genreAssoc struct {
			ID       uuid.UUID `db:"id"`
			SeriesID uuid.UUID `db:"series_id"`
			GenreID  int       `db:"genre_id"`
		}
		genreAssocs := make([]genreAssoc, len(genres))
		for k, v := range genres {
			genreAssocs[k] = genreAssoc{uuid.New(), seriesID, v.Id}
		}

		if err := database.InExec(db, `DELETE FROM series_genres sg WHERE sg.series_id=$1`, seriesID); err != nil {
			return err
		}

		_, err := db.NamedExec(`
			INSERT INTO series_genres(id, series_id, genre_id)
			VALUES(:id, :series_id, :genre_id)
			ON CONFLICT(series_id, genre_id) DO NOTHING
		`, genreAssocs)

		return err
	}

	_, err := db.Exec(`
		DELETE FROM series_genres WHERE series_id=$1`, seriesID)
	return err
}

// SaveSeason upserts the provided Season model to the database. Existing models
// to update are found using the 'TmdbID' as this is expected to be a stable
// identifier.
//
// NOTE: the PK and FK ID's of the media may be UPDATED to match existing DB entry (if any)
func (store *Store) SaveSeason(db database.Queryable, season *Season) error {
	var updatedSeason Season
	if err := db.QueryRowx(`
		INSERT INTO season(id, tmdb_id, season_number, title, series_id, created_at, updated_at)
		VALUES($1, $2, $3, $4, $5, current_timestamp, current_timestamp)
		ON CONFLICT(tmdb_id) DO UPDATE
			SET (season_number, title, series_id, updated_at) = (EXCLUDED.season_number, EXCLUDED.title, EXCLUDED.series_id, current_timestamp)
		RETURNING *
	`, season.ID, season.TmdbID, season.SeasonNumber, season.Title, season.SeriesID).StructScan(&updatedSeason); err != nil {
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
func (store *Store) SaveEpisode(db database.Queryable, episode *Episode) error {
	var updatedEpisode Episode
	if err := db.QueryRowx(`
		INSERT INTO media(id, type, tmdb_id, episode_number, title, source_path, season_id, adult, created_at, updated_at)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, current_timestamp, current_timestamp)
		ON CONFLICT(tmdb_id, type) DO UPDATE
			SET (episode_number, title, source_path, season_id, updated_at, adult) =
				(EXCLUDED.episode_number, EXCLUDED.title, EXCLUDED.source_path, EXCLUDED.season_id, current_timestamp, EXCLUDED.adult)
		RETURNING id, tmdb_id, episode_number, title, source_path, season_id, adult, created_at, updated_at;
	`, episode.ID, "episode", episode.TmdbID, episode.EpisodeNumber, episode.Title, episode.SourcePath, episode.SeasonID, episode.Adult).StructScan(&updatedEpisode); err != nil {
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
func (store *Store) GetMedia(db database.Queryable, mediaID uuid.UUID) *Container {
	if movie, err := store.GetMovie(db, mediaID); err != nil {
		//TODO: consider wrapping these three in a transaction (probably overkill though)
		storeLogger.Emit(logger.DEBUG, "Failed to find movie with media ID %s: %v {falling back to searching for episode}\n", mediaID, err)
		if episode, err := store.GetEpisode(db, mediaID); err != nil {
			storeLogger.Emit(logger.DEBUG, "Failed to fetch episode with media ID %s: %v\n", mediaID, err)
			return nil
		} else {
			season, err := store.GetSeason(db, episode.SeasonID)
			if err != nil {
				storeLogger.Emit(logger.FATAL, "Episode %s found, but error (%v) occurred when fetching referenced season. This may indicate a serious problem with the referential integrity of the DB\n", mediaID, err)
				return nil
			}
			series, err := store.GetSeries(db, season.SeriesID)
			if err != nil {
				storeLogger.Emit(logger.FATAL, "Episode %s and season %s found, but error (%v) occurred when fetching referenced series. This may indicate a serious problem with the referential integrity of the DB\n", mediaID, season.ID, err)
				return nil
			}
			return &Container{Type: EPISODE, Episode: episode, Series: series, Season: season}
		}
	} else {
		return &Container{Type: MOVIE, Movie: movie}
	}
}

// ListMovie returns the Movie models for all media of type 'movie' in the database, or an error
// if the underpinning SQL query failed
func (store *Store) ListMovie(db *sqlx.DB) ([]*Movie, error) {
	var dest []*Movie
	if err := db.Unsafe().Select(&dest, `SELECT * FROM media WHERE type='movie'`); err != nil {
		return nil, fmt.Errorf("failed to select all movies: %v", err)
	}

	return dest, nil
}

// ListSeries returns the Series models for series stored in the database, or an error
// if the underpinning SQL query failed
func (store *Store) ListSeries(db database.Queryable) ([]*Series, error) {
	var dest []*Series
	if err := db.Select(&dest, `SELECT * FROM series`); err != nil {
		return nil, fmt.Errorf("failed to select all series: %v", err)
	}

	return dest, nil
}

func (result *MediaListResult) IsMovie() bool  { return result.Movie != nil && result.Series == nil }
func (result *MediaListResult) IsSeries() bool { return result.Movie == nil && result.Series != nil }

func (ord *MediaListOrderBy) String() string {
	dir := "ASC"
	if ord.Descending {
		dir = "DESC"
	}

	return fmt.Sprintf("%s %s", ord.Column, dir)
}

const (
	MovieType  MediaListType = "movie"
	SeriesType MediaListType = "series"
)

const (
	IDColumn        MediaListOrderColumn = "id" // stable identifier for 'unsorted' media
	UpdatedAtColumn MediaListOrderColumn = "updated_at"
	CreatedAtColumn MediaListOrderColumn = "created_at"
	TitleColumn     MediaListOrderColumn = "title"
)

// ListMedia allows for series/movies to be listed (controllable using allowedTypes). The query also
// allows for an offset/limit to be provided, facilitating simple paging of the results.
//   - allowedTypes -> defaults to movies and series
//   - orderBy -> defaults to updated_at in ascending order
//   - offset -> defaults to 0
//   - limit -> default to 15, maximum 100
func (store *Store) ListMedia(db database.Queryable, allowedTypes []MediaListType, orderBy []MediaListOrderBy, offset int, limit int) ([]*MediaListResult, error) {
	if len(allowedTypes) == 0 {
		allowedTypes = []MediaListType{"movie", "series"}
	}

	movieEnabledClause := "AND false"
	seriesAllowedClause := "WHERE false"
	for _, v := range allowedTypes {
		switch v {
		case MovieType:
			movieEnabledClause = ""
		case SeriesType:
			seriesAllowedClause = ""
		}
	}

	limitClause := 15
	if limit > 0 {
		limitClause = min(limit, 100)
	}

	orderByClause := "updated_at ASC"
	if len(orderBy) > 0 {
		b := orderBy[0].String()
		for _, s := range orderBy[1:] {
			b = b + "," + s.String()
		}
		orderByClause = b
	}

	query := fmt.Sprintf(`
		WITH joinedMedia(type, id, title, tmdb_id, created_at, updated_at, series_season_count, genres) AS (
			SELECT 
				'movie' AS type,
				id,
				title,
				tmdb_id,
				created_at,
				updated_at,
				0,
				(
					SELECT COALESCE(JSONB_AGG(DISTINCT genre.*) FILTER (WHERE genre.id IS NOT NULL), '[]')
					FROM movie_genres mg
					INNER JOIN genre
					ON genre.id = mg.genre_id
					WHERE mg.movie_id = media.id
				)
			FROM media WHERE type='movie' %s 
			UNION
			SELECT
				'series' AS type,
				id,
				title,
				tmdb_id,
				created_at,
				updated_at,
				(SELECT COUNT(*) FROM season WHERE season.series_id = series.id),
				(
					SELECT COALESCE(JSONB_AGG(DISTINCT genre.*) FILTER (WHERE genre.id IS NOT NULL), '[]')
					FROM series_genres sg
					INNER JOIN genre
					ON genre.id = sg.genre_id
					WHERE sg.series_id = series.id
				)
			FROM series
			%s
		)
		SELECT *
		FROM joinedMedia
		ORDER BY %s
		OFFSET %d
		LIMIT %d
	`, movieEnabledClause, seriesAllowedClause, orderByClause, max(offset, 0), limitClause)

	var results []struct {
		ID          uuid.UUID            `db:"id"`
		Title       string               `db:"title"`
		TmdbID      string               `db:"tmdb_id"`
		CreatedAt   time.Time            `db:"created_at"`
		UpdatedAt   time.Time            `db:"updated_at"`
		SeasonCount int                  `db:"series_season_count"`
		MediaType   string               `db:"type"`
		Genres      jsonColumn[[]*Genre] `db:"genres"`
	}
	if err := db.Select(&results, query); err != nil {
		return nil, err
	}

	out := make([]*MediaListResult, len(results))
	for k, v := range results {
		model := Model{ID: v.ID, TmdbID: v.TmdbID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, Title: v.Title}
		switch v.MediaType {
		case "movie":
			out[k] = &MediaListResult{Movie: &Movie{Model: model, Genres: *v.Genres.Get()}}
		case "series":
			out[k] = &MediaListResult{Series: &SeriesStub{Series: &Series{Model: model, Genres: *v.Genres.Get()}, SeasonCount: v.SeasonCount}}
		default:
			return nil, fmt.Errorf("type of list result %v is illegal. Expected 'movie' or 'series', found '%s'", v, v.MediaType)
		}
	}

	return out, nil
}

// SaveGenres saves the given genre labels to the database, ignoring any which
// already exist in the database (determined based on label conflicts). This function
// will return back all the genres referenced by this query, either as a result
// of insertion or because they were already inside the database.
//
// NB: This query should be executed inside of a transaction
func (store *Store) SaveGenres(db database.Queryable, genres []*Genre) ([]*Genre, error) {
	if len(genres) == 0 {
		return []*Genre{}, nil
	}

	if _, err := db.NamedExec(
		`INSERT INTO genre(label) VALUES (:label) ON CONFLICT(label) DO NOTHING`,
		genres,
	); err != nil {
		return nil, fmt.Errorf("failed to insert bulk genres: %w", err)
	}

	query, args, err := sqlx.Named(`SELECT * FROM genre WHERE label = any(:label)`, genres)
	if err != nil {
		return nil, fmt.Errorf("failed to construct named query: %w", err)
	}

	var results []*Genre
	if err := db.Select(&results, db.Rebind(query), pq.Array(args)); err != nil {
		return nil, fmt.Errorf("failed to select saved genres: %w [query %s and args %#v]", err, query, args)
	}

	return results, nil
}

func (store *Store) ListGenres(db database.Queryable) ([]*Genre, error) {
	var results []*Genre
	if err := db.Select(&results, `SELECT * FROM genre`); err != nil {
		return nil, err
	}

	return results, nil
}

func (store *Store) GetGenresForMovie(db database.Queryable, movieID uuid.UUID) ([]*Genre, error) {
	var results []*Genre
	if err := db.Select(&results, `
		SELECT genre.* FROM movie_genres
		INNER JOIN genre
		ON genre.id = movie_genres.genre_id
		WHERE movie_genres.movie_id = $1
		`, movieID); err != nil {
		return nil, err
	}

	return results, nil
}

func (store *Store) GetGenresForSeries(db database.Queryable, seriesID uuid.UUID) ([]*Genre, error) {
	var results []*Genre
	if err := db.Select(&results, `
		SELECT genre.* FROM series_genres
		INNER JOIN genre
		ON genre.id = series_genres.genre_id
		WHERE series_genres.series_id = $1
		`, seriesID); err != nil {
		return nil, err
	}

	return results, nil
}

// CountSeasonsInSeries queries the database for the number of seasons associated with
// each of the given series, and constructs a mapping from seriesID -> season count.
// NB: series which did not exist in the database will be omitted from the result mapping
func (store *Store) CountSeasonsInSeries(db database.Queryable, seriesIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	query, args, err := sqlx.In(`
		SELECT series.id AS id, COUNT(season.*) AS count FROM series
		LEFT JOIN season
		  ON season.series_id = series.id
		WHERE series.id IN (?)
		GROUP BY series.id`, seriesIDs)

	if err != nil {
		return nil, fmt.Errorf("failed to construct query to count seasons for series %v: %v", seriesIDs, err)
	}

	type r struct {
		ID    uuid.UUID `db:"id"`
		Count int       `db:"count"`
	}

	var results []*r
	if err := db.Select(&results, db.Rebind(query), args...); err != nil {
		return nil, fmt.Errorf("failed to count seasons asscoiated with series %v: %v", seriesIDs, err)
	}

	finalResult := make(map[uuid.UUID]int)
	for _, v := range results {
		finalResult[v.ID] = v.Count
	}

	return finalResult, nil
}

// GetSeasonsForSeries queries the database for all seasons which are 'owned' by the series
// referenced by the ID specified. If the ID provided does not match a known series, or if that
// series has no seasons, the result will be an empty slice.
func (store *Store) GetSeasonsForSeries(db database.Queryable, seriesID uuid.UUID) ([]*Season, error) {
	var dest []*Season
	if err := db.Select(&dest, `
		SELECT season.* FROM series
     	LEFT JOIN season
	      ON season.series_id = series.id
	    WHERE series.id=$1`,
		seriesID,
	); err != nil {
		return nil, fmt.Errorf("failed to fetch seasons for series %s: %w", seriesID, err)
	}

	return dest, nil
}

// GetEpisodesForSeries accepts a list of series IDs and queries the database
// for all the episodes referencing them (indirectly, via the associated seasons).
// The result is constructed in to a map such that each key is one of the
// series IDs, and the value is a slice of all the
// episodes related to that series.
//
// NB: if a series ID does not reference an existing series, or it has no episodes, then it's key
// will be missing from the resulting map.
func (store *Store) GetEpisodesForSeries(db database.Queryable, seriesIDs []uuid.UUID) (map[uuid.UUID][]*Episode, error) {
	wrap := func(err error) error {
		return fmt.Errorf("failed to get episodes for series %s: %w", seriesIDs, err)
	}

	query, args, err := sqlx.In(`
		SELECT series.id AS owning_series_id, media.* FROM series
		INNER JOIN season
		  ON season.series_id = series.id
		INNER JOIN media
		  ON media.type = 'episode'
		 AND media.season_id = season.id
		WHERE series.id IN (?)`, seriesIDs)
	if err != nil {
		return nil, wrap(err)
	}

	type r struct {
		media
		OwningSeriesID uuid.UUID `db:"owning_series_id"`
	}

	var dest []*r
	if err := db.Select(&dest, db.Rebind(query), args...); err != nil {
		return nil, wrap(err)
	}

	output := make(map[uuid.UUID][]*Episode)
	for _, v := range dest {
		output[v.OwningSeriesID] = append(output[v.OwningSeriesID], mediaToEpisode(&v.media))
	}

	return output, nil
}

// GetEpisodesForSeasons accepts a list of season IDs and queries the database
// for all the episodes referencing them. The result is constructed in to a map
// such that each key is one of the season IDs, and the value is a slice of all the
// episodes related to that season.
//
// NB: if a season ID does not reference an existing season, or it has no episodes, then it's key
// will be missing from the resulting map.
func (store *Store) GetEpisodesForSeasons(db database.Queryable, seasonIDs []uuid.UUID) (map[uuid.UUID][]*Episode, error) {
	wrap := func(err error) error {
		return fmt.Errorf("failed to get episodes for seasons %s: %w", seasonIDs, err)
	}

	query, args, err := sqlx.In(`
		SELECT season.id AS owning_season_id, media.* FROM season
     	INNER JOIN media
	      ON media.type = 'episode'
		 AND media.season_id = season.id
	    WHERE season.id IN (?)`, seasonIDs)
	if err != nil {
		return nil, wrap(err)
	}

	type r struct {
		media
		OwningSeasonID uuid.UUID `db:"owning_season_id"`
	}

	var dest []*r
	if err := db.Select(&dest, db.Rebind(query), args...); err != nil {
		return nil, wrap(err)
	}

	output := make(map[uuid.UUID][]*Episode)
	for _, v := range dest {
		output[v.OwningSeasonID] = append(output[v.OwningSeasonID], mediaToEpisode(&v.media))
	}

	return output, nil
}

// GetMovie searches for an existing movie with the Thea PK ID provided.
func (store *Store) GetMovie(db database.Queryable, movieID uuid.UUID) (*Movie, error) {
	return queryRowMovie(db, MediaTable, IDCol, movieID)
}

// GetMovieWithTmdbId searches for an existing movie with the TMDB unique ID provided.
func (store *Store) GetMovieWithTmdbId(db database.Queryable, tmdbID string) (*Movie, error) {
	return queryRowMovie(db, MediaTable, TmdbIDCol, tmdbID)
}

// GetSeries searches for an existing series with the Thea PK ID provided.
func (store *Store) GetSeries(db database.Queryable, seriesID uuid.UUID) (*Series, error) {
	return queryRow[Series](db, SeriesTable, IDCol, seriesID, "")
}

// GetSeriesWithTmdbId searches for an existing series with the TMDB unique ID provided.
func (store *Store) GetSeriesWithTmdbId(db database.Queryable, tmdbID string) (*Series, error) {
	return queryRow[Series](db, SeriesTable, TmdbIDCol, tmdbID, "")
}

// GetSeason searches for an existing season with the Thea PK ID provided.
func (store *Store) GetSeason(db database.Queryable, seasonID uuid.UUID) (*Season, error) {
	return queryRow[Season](db, SeasonTable, IDCol, seasonID, "")
}

// GetSeasonWithTmdbId searches for an existing season with the TMDB unique ID provided.
func (store *Store) GetSeasonWithTmdbId(db database.Queryable, tmdbID string) (*Season, error) {
	return queryRow[Season](db, SeasonTable, TmdbIDCol, tmdbID, "")
}

// GetEpisode searches for an existing episode with the Thea PK ID provided.
func (store *Store) GetEpisode(db database.Queryable, episodeID uuid.UUID) (*Episode, error) {
	return queryRowEpisode(db, MediaTable, IDCol, episodeID)
}

// GetEpisodeWithTmdbId searches for an existing episode with the TMDB unique ID provided.
func (store *Store) GetEpisodeWithTmdbId(db database.Queryable, tmdbID string) (*Episode, error) {
	return queryRowEpisode(db, MediaTable, TmdbIDCol, tmdbID)
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

// DeleteSeries deletes the series with the given ID, including all it's seasons and
// enclosed episodes.
//
// NB: It is important to explicitly delete associated media transcodes for the affected
// episodes before attempting to delete this resource - failure to do so will cause
// this query to fail.
func (store *Store) DeleteSeries(db database.Queryable, seriesID uuid.UUID) error {
	if _, err := db.Exec(`DELETE FROM series WHERE id=$1`, seriesID); err != nil {
		return fmt.Errorf("deletion of series %s failed: %w", seriesID, err)
	}

	return nil
}

// DeleteSeason deletes the series with the given ID, including all it's enclosed episodes.
//
// NB: It is important to explicitly delete associated media transcodes for the affected
// episodes before attempting to delete this resource - failure to do so will cause
// this query to fail.
func (store *Store) DeleteSeason(db database.Queryable, seasonID uuid.UUID) error {
	if _, err := db.Exec(`DELETE FROM season WHERE id=$1`, seasonID); err != nil {
		return fmt.Errorf("deletion of season %s failed: %w", seasonID, err)
	}

	return nil
}

// DeleteEpisode deletes the episode with the given ID
//
// NB: It is important to explicitly delete associated media transcodes for the affected
// episode before attempting to delete this resource - failure to do so will cause
// this query to fail.
func (store *Store) DeleteEpisode(db database.Queryable, episodeID uuid.UUID) error {
	if _, err := db.Exec(`DELETE FROM media WHERE type='episode' AND id=$1`, episodeID); err != nil {
		return fmt.Errorf("deletion of episode %s failed: %w", episodeID, err)
	}

	return nil
}

// DeleteMovie deletes the movie with the given ID
//
// NB: It is important to explicitly delete associated media transcodes for the affected
// movie before attempting to delete this resource - failure to do so will cause
// this query to fail.
func (store *Store) DeleteMovie(db database.Queryable, movieID uuid.UUID) error {
	if _, err := db.Exec(`DELETE FROM media WHERE type='movie' AND id=$1`, movieID); err != nil {
		return fmt.Errorf("deletion of movie %s failed: %w", movieID, err)
	}

	return nil
}

// queryRowMovie extracts a Media row from the database and ensures that the row returned represents
// a movie (the type must be 'movie', and episode-specific information must be nil).
func queryRowMovie(db database.Queryable, table string, col string, val any) (*Movie, error) {
	r, e := queryRow[media](db, table, col, val, MediaMovieClause)
	if e != nil {
		return nil, e
	}

	if r.Type != "movie" || r.EpisodeNumber != nil || r.SeasonID != nil {
		return nil, fmt.Errorf("media query for an episode returned malformed data expected ('movie', nil, nil), found (%v, %v, %v)", r.Type, r.EpisodeNumber, r.SeasonID)
	}

	return &Movie{
		Model:     r.Model,
		Watchable: r.Watchable,
	}, nil
}

// queryRowEpisode extracts a Media row from the database and ensures that the row returned represents
// an episode (the type must be 'episode', and the episode-specific information must be non-nil)
func queryRowEpisode(db database.Queryable, table string, col string, val any) (*Episode, error) {
	r, e := queryRow[media](db, table, col, val, MediaEpisodeClause)
	if e != nil {
		return nil, e
	}

	if r.Type != "episode" || r.EpisodeNumber == nil || r.SeasonID == nil {
		return nil, fmt.Errorf("media query for an episode returned malformed data expected ('episode', non-nil, non-nil), found (%v, %v, %v)", r.Type, r.EpisodeNumber, r.SeasonID)
	}

	return mediaToEpisode(r), nil
}

// queryRow selects a single row from the given table using a where clause constructed
// from the col and val provided (i.e. WHERE col=val). An additionalWhereClause may be
// provided as well which is appended afterwards (and as such, the additional clause must
// begin with 'AND ...').
// If zero rows are returned, then 'ErrNoRowFound' is returned.
func queryRow[T any](db database.Queryable, table string, col string, val any, additionalWhereClause string) (*T, error) {
	var dest T
	query := fmt.Sprintf(`SELECT * FROM %s WHERE %s=$1 %s LIMIT 1;`, table, col, additionalWhereClause)
	if err := db.Get(&dest, query, val); err != nil {
		return nil, fmt.Errorf("query for %s failed: %w", table, err)
	}

	return &dest, nil
}

func mediaToEpisode(m *media) *Episode {
	return &Episode{
		Model:         m.Model,
		Watchable:     m.Watchable,
		SeasonID:      *m.SeasonID,
		EpisodeNumber: *m.EpisodeNumber,
	}
}
