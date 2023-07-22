package media

import (
	"time"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
)

type (
	// Model contains the union of properties that we expect all store-able information
	// to contain. This is typically basic information about the container.
	Model struct {
		ID        uuid.UUID
		TmdbId    string // unique
		CreatedAt time.Time
		UpdatedAt time.Time
		Title     string
	}

	// Watchable represents the union of properties that we expect to see
	// populated on all watchable media (movie/episode). Media containers,
	// such as a series/season are not required to contain this information
	Watchable struct {
		MediaResolution
		SourcePath string
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
		SeasonNumber int
		Episodes     []Episode
		SeriesID     uuid.UUID
	}

	// Series represents the information Thea stores about a series. A one-to-many
	// relationship exists between series and seasons, although the seasons themselves
	// are not contained within this model.
	Series struct {
		Model
		Adult   bool
		Seasons []Season
	}

	// Episode contains all the information unique to an episode, combined
	// with the 'Common' struct.
	Episode struct {
		Model
		Watchable
		SeasonID      uuid.UUID
		EpisodeNumber int
	}

	Movie struct {
		Model
		Watchable
		Adult bool
	}

	Store struct{}
)

func (store *Store) RegisterModels(db database.Manager) {
	db.RegisterModels(Movie{}, Episode{}, Series{}, Season{})
}

// SaveMovie upserts the provided Movie model to the database. Existing models
// to update are found using the 'TmdbId' as this is expected to be a stable
// identifier.
//
// NOTE: the ID of the media may be UPDATED to match existing DB entry (if any)
func (store *Store) SaveMovie(db database.Goqu, movie *Movie) error {
	// movieID := movie.ID

	// var existingMovie *Movie
	// db.Where(Movie{Model: Model{TmdbId: movie.TmdbId}}).First(&existingMovie)
	// if existingMovie.ID != uuid.Nil {
	// 	movie.ID = existingMovie.ID
	// }

	// err := db.Debug().Save(movie).Error
	// if err != nil {
	// 	movie.ID = movieID
	// }

	// return err
	return nil
}

// SaveSeries upserts the provided Series model to the database. Existing models
// to update are found using the 'TmdbID' as this is expected to be a stable
// identifier.
//
// NOTE: the ID of the media may be UPDATED to match existing DB entry (if any)
func (store *Store) SaveSeries(db database.Goqu, series *Series) error {
	// seriesID := series.ID

	// var existingSeries Series
	// db.Where(Series{Model: Model{TmdbId: series.TmdbId}}).First(&existingSeries)
	// if existingSeries.ID != uuid.Nil {
	// 	series.ID = existingSeries.ID
	// }

	// err := db.Debug().Save(series).Error
	// if err != nil {
	// 	series.ID = seriesID
	// }

	// return err
	return nil
}

// SaveSeason upserts the provided Season model to the database. Existing models
// to update are found using the 'TmdbID' as this is expected to be a stable
// identifier.
//
// NOTE: the ID of the media may be UPDATED to match existing DB entry (if any)
func (store *Store) SaveSeason(db database.Goqu, season *Season) error {
	// seasonID := season.ID

	// var existingSeason *Season
	// db.Where(&Season{Model: Model{TmdbId: season.TmdbId}}).First(&existingSeason)
	// if existingSeason.ID != uuid.Nil {
	// 	season.ID = existingSeason.ID
	// }

	// err := db.Debug().Save(season).Error
	// if err != nil {
	// 	season.ID = seasonID
	// }

	// return err
	return nil
}

// saveEpisode transactionally upserts the episode and it's season
// and series. Existing models are found using the models 'TmdbID'
// as this is expected to be a stable identifier.
//
// NOTE: the ID of the media(s) may be UPDATED to match existing DB entry (if any)
func (store *Store) SaveEpisode(db database.Goqu, episode *Episode) error {
	// Store old PKs so we can rollback on transaction failure
	// episodeID := episode.ID

	// var existingEpisode *Episode
	// db.Where(&Episode{Model: Model{TmdbId: episode.TmdbId}}).First(&existingEpisode)
	// if existingEpisode.ID != uuid.Nil {
	// 	episode.ID = existingEpisode.ID
	// }

	// err := db.Debug().Save(episode).Error
	// if err != nil {
	// 	episode.ID = episodeID
	// }

	// return err
	return nil
}

// GetMedia is a convinience method for requesting either a Movie
// or an Episode. The ID provided is used to lookup both, and whichever
// query is successful is used to populate a media Container.
func (store *Store) GetMedia(db database.Goqu, mediaID uuid.UUID) *Container {
	if _, err := store.GetMovie(db, mediaID); err != nil {
		if _, err := store.GetEpisode(db, mediaID); err != nil {
			return nil
		} else {
			// return &Container{
			// 	Type:    EPISODE,
			// 	Episode: episode,
			// 	Movie:   nil,
			// }
			return nil
		}
	} else {
		return nil
		// return &Container{
		// 	Type:    MOVIE,
		// 	Movie:   movie,
		// 	Episode: nil,
		// }
	}
}

// GetMovie searches for an existing movie with the Thea PK ID provided.
func (store *Store) GetMovie(db database.Goqu, movieID uuid.UUID) (*Movie, error) {
	return store.getMovie(db, Movie{Model: Model{ID: movieID}})
}

// GetMovieWithTmdbId searches for an existing movie with the TMDB unique ID provided.
func (store *Store) GetMovieWithTmdbId(db database.Goqu, movieID string) (*Movie, error) {
	return store.getMovie(db, Movie{Model: Model{TmdbId: movieID}})
}

// getMovie will search the database for a Movie row matching the
// model provided. No result will cause 'nil' to be returned, failure
// for any other reason will see 'nil' returned.
func (store *Store) getMovie(db database.Goqu, searchModel Movie) (*Movie, error) {
	// var result Movie
	// err := db.Where(searchModel).First(&result).Error
	// if err != nil {
	// 	return nil, err
	// }

	// return &result, nil
	return nil, nil
}

// GetSeries searches for an existing series with the Thea PK ID provided.
func (store *Store) GetSeries(db database.Goqu, movieID uuid.UUID) (*Series, error) {
	return store.getSeries(db, Series{Model: Model{ID: movieID}})
}

// GetSeriesWithTmdbId searches for an existing series with the TMDB unique ID provided.
func (store *Store) GetSeriesWithTmdbId(db database.Goqu, movieID string) (*Series, error) {
	return store.getSeries(db, Series{Model: Model{TmdbId: movieID}})
}

// getSeries will search the database for a Series row matching the
// PK ID provided. No result will cause 'nil' to be returned, failure
// for any other reason will see 'nil' returned.
func (store *Store) getSeries(db database.Goqu, searchModel Series) (*Series, error) {
	// var result Series
	// err := db.Where(searchModel).First(&result).Error
	// if err != nil {
	// 	return nil, err
	// }

	// return &result, nil
	return nil, nil
}

// GetSeason searches for an existing season with the Thea PK ID provided.
func (store *Store) GetSeason(db database.Goqu, movieID uuid.UUID) (*Season, error) {
	return store.getSeason(db, Season{Model: Model{ID: movieID}})
}

// GetSeasonWithTmdbId searches for an existing season with the TMDB unique ID provided.
func (store *Store) GetSeasonWithTmdbId(db database.Goqu, movieID string) (*Season, error) {
	return store.getSeason(db, Season{Model: Model{TmdbId: movieID}})
}

// getSeason will search the database for a Season row matching the
// PK ID provided. No result will cause 'nil' to be returned, failure
// for any other reason will see 'nil' returned.
func (store *Store) getSeason(db database.Goqu, searchModel Season) (*Season, error) {
	// var result Season
	// err := db.Where(searchModel).First(&result).Error
	// if err != nil {
	// 	return nil, err
	// }

	// return &result, nil
	return nil, nil
}

// GetEpisode searches for an existing episode with the Thea PK ID provided.
func (store *Store) GetEpisode(db database.Goqu, movieID uuid.UUID) (*Episode, error) {
	return store.getEpisode(db, Episode{Model: Model{ID: movieID}})
}

// GetEpisodeWithTmdbId searches for an existing episode with the TMDB unique ID provided.
func (store *Store) GetEpisodeWithTmdbId(db database.Goqu, movieID string) (*Episode, error) {
	return store.getEpisode(db, Episode{Model: Model{TmdbId: movieID}})
}

// getEpisode will search the database for a Episode row matching the
// PK ID provided. No result will cause 'nil' to be returned, failure
// for any other reason will see 'nil' returned.
func (store *Store) getEpisode(db database.Goqu, searchModel Episode) (*Episode, error) {
	// var result Episode
	// err := db.Where(searchModel).First(&result).Error
	// if err != nil {
	// 	return nil, err
	// }

	// return &result, nil
	return nil, nil
}

// GetAllSourcePaths returns all the source paths related
// to media that is currently known to Thea by polling the database.
func (store *Store) GetAllSourcePaths(db database.Goqu) []string {
	return make([]string, 0)
}
