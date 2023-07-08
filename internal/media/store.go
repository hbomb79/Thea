package media

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"gorm.io/gorm"
)

type (
	// Season represents the information Thea stores about a season
	// of episodes itself. A season is related to many episodes (however
	// this payload does not contain them); additionally, a series is related
	// to many seasons.
	Season struct {
		Id        uuid.UUID `gorm:"primaryKey"`
		CreatedAt time.Time
		UpdatedAt time.Time
		TmdbId    string `gorm:"uniqueIndex"`
	}

	// Series represents the information Thea stores about a series. A one-to-many
	// relationship exists between series and seasons, although the seasons themselves
	// are not contained within this struct.
	Series struct {
		Id        uuid.UUID `gorm:"primaryKey"`
		CreatedAt time.Time
		UpdatedAt time.Time
		TmdbId    string `gorm:"uniqueIndex"`
	}

	// Episode contains all the information unique to an episode, combined
	// with the 'Common' struct.
	Episode struct {
		SeasonNumber  int
		EpisodeNumber int
		Common
	}

	Movie struct {
		Common
	}

	Common struct {
		Id         uuid.UUID `gorm:"primaryKey"`
		CreatedAt  time.Time
		UpdatedAt  time.Time
		Title      string
		Resolution int
		SourcePath string
		TmdbId     string `gorm:"uniqueIndex"`
	}

	Store struct {
		db database.Manager
	}
)

// NewStore uses the provided DB manager to register
// the models that this store defines, before storing
// a reference to the manager for use later when
// performing queries.
//
// Note: The manager provided is expected to NOT be
// connected, and it is expected to have become
// connected before any other store methods are used.
func NewStore(db database.Manager) (*Store, error) {
	if db.GetInstance() != nil {
		return nil, errors.New("database is already connected")
	}

	db.RegisterModels(Movie{}, Episode{}, Series{}, Season{})
	return &Store{db: db}, nil
}

// SaveMovie upserts the provided Movie model to the database. Existing models
// to update are found using the 'TmdbId' as this is expected to be a stable
// identifier.
//
// NOTE: the ID of the media may be UPDATED to match existing DB entry (if any)
func (store *Store) SaveMovie(movie *Movie) error {
	return saveMovie(store.db.GetInstance(), movie)
}

// SaveSeries upserts the provided Series model to the database. Existing models
// to update are found using the 'TmdbId' as this is expected to be a stable
// identifier.
//
// NOTE: the ID of the media may be UPDATED to match existing DB entry (if any)
func (store *Store) SaveSeries(series *Series) error {
	return saveSeries(store.db.GetInstance(), series)
}

// SaveSeason upserts the provided Season model to the database. Existing models
// to update are found using the 'TmdbId' as this is expected to be a stable
// identifier.
//
// NOTE: the ID of the media may be UPDATED to match existing DB entry (if any)
func (store *Store) SaveSeason(season *Season) error {
	return saveSeason(store.db.GetInstance(), season)
}

// saveEpisode transactionally upserts the episode and it's season
// and series. Existing models are found using the models 'TmdbId'
// as this is expected to be a stable identifier.
//
// NOTE: the ID of the media(s) may be UPDATED to match existing DB entry (if any)
func (store *Store) SaveEpisode(episode *Episode, season *Season, series *Series) error {
	// Store old PKs so we can rollback on transaction failure
	episodeId := episode.Id
	seasonId := season.Id
	seriesId := series.Id

	if err := store.db.GetInstance().Transaction(func(tx *gorm.DB) error {
		if err := saveSeries(tx, series); err != nil {
			return err
		}

		if err := saveSeason(tx, season); err != nil {
			return err
		}

		if err := saveEpisode(tx, episode); err != nil {
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

// GetMedia is a convinience method for requesting either a Movie
// or an Episode. The ID provided is used to lookup both, and whichever
// query is successful is used to populate a media Container.
func (store *Store) GetMedia(mediaId uuid.UUID) *Container {
	if movie, err := store.GetMovie(mediaId); err != nil {
		if episode, err := store.GetEpisode(mediaId); err != nil {
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

// GetMovie will search the database for a Movie row matching the
// PK ID provided. No result will cause 'nil' to be returned, failure
// for any other reason will see 'nil' returned.
func (store *Store) GetMovie(movieId uuid.UUID) (*Movie, error) {
	var result Movie
	err := store.db.GetInstance().Where(&Movie{Common: Common{Id: movieId}}).First(&result).Error
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// GetEpisode will search the database for a Episode row matching the
// PK ID provided. No result will cause 'nil' to be returned, failure
// for any other reason will see 'nil' returned.
func (store *Store) GetEpisode(episodeId uuid.UUID) (*Episode, error) {
	var result Episode
	err := store.db.GetInstance().Where(&Episode{Common: Common{Id: episodeId}}).First(&result).Error
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// GetSeason will search the database for a Season row matching the
// PK ID provided. No result will cause 'nil' to be returned, failure
// for any other reason will see 'nil' returned.
func (store *Store) GetSeason(seasonId uuid.UUID) (*Season, error) {
	var result Season
	err := store.db.GetInstance().Where(&Season{Id: seasonId}).First(&result).Error
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// GetSeries will search the database for a Series row matching the
// PK ID provided. No result will cause 'nil' to be returned, failure
// for any other reason will see 'nil' returned.
func (store *Store) GetSeries(seriesId uuid.UUID) (*Series, error) {
	var result Series
	err := store.db.GetInstance().Where(&Series{Id: seriesId}).First(&result).Error
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// GetAllSourcePaths returns all the source paths related
// to media that is currently known to Thea by polling the database.
func (store *Store) GetAllSourcePaths() []string {
	return make([]string, 0)
}

func saveMovie(db *gorm.DB, movie *Movie) error {
	movieId := movie.Id

	var existingMovie *Movie
	db.Where(&Movie{Common: Common{TmdbId: movie.TmdbId}}).First(&existingMovie)
	if existingMovie != nil {
		movie.Id = existingMovie.Id
	}

	err := db.Debug().Save(movie).Error
	if err != nil {
		movie.Id = movieId
	}

	return err
}

func saveEpisode(db *gorm.DB, episode *Episode) error {
	episodeId := episode.Id

	var existingEpisode *Episode
	db.Where(&Episode{Common: Common{TmdbId: episode.TmdbId}}).First(&existingEpisode)
	if existingEpisode != nil {
		episode.Id = existingEpisode.Id
	}

	err := db.Debug().Save(episode).Error
	if err != nil {
		episode.Id = episodeId
	}

	return err
}

func saveSeries(db *gorm.DB, series *Series) error {
	seriesId := series.Id

	var existingSeries *Series
	db.Where(&Series{TmdbId: series.TmdbId}).First(&existingSeries)
	if existingSeries != nil {
		series.Id = existingSeries.Id
	}

	err := db.Debug().Save(series).Error
	if err != nil {
		series.Id = seriesId
	}

	return nil
}

func saveSeason(db *gorm.DB, season *Season) error {
	seasonId := season.Id

	var existingSeason *Season
	db.Where(&Season{TmdbId: season.TmdbId}).First(&existingSeason)
	if existingSeason != nil {
		season.Id = existingSeason.Id
	}

	err := db.Debug().Save(season).Error
	if err != nil {
		season.Id = seasonId
	}

	return nil
}
