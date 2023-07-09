package media

import (
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
func (store *Store) SaveMovie(db *gorm.DB, movie *Movie) error {
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

// SaveSeries upserts the provided Series model to the database. Existing models
// to update are found using the 'TmdbId' as this is expected to be a stable
// identifier.
//
// NOTE: the ID of the media may be UPDATED to match existing DB entry (if any)
func (store *Store) SaveSeries(db *gorm.DB, series *Series) error {
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

	return err
}

// SaveSeason upserts the provided Season model to the database. Existing models
// to update are found using the 'TmdbId' as this is expected to be a stable
// identifier.
//
// NOTE: the ID of the media may be UPDATED to match existing DB entry (if any)
func (store *Store) SaveSeason(db *gorm.DB, season *Season) error {
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

	return err
}

// saveEpisode transactionally upserts the episode and it's season
// and series. Existing models are found using the models 'TmdbId'
// as this is expected to be a stable identifier.
//
// NOTE: the ID of the media(s) may be UPDATED to match existing DB entry (if any)
func (store *Store) SaveEpisode(db *gorm.DB, episode *Episode, season *Season, series *Series) error {
	// Store old PKs so we can rollback on transaction failure
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

// GetMedia is a convinience method for requesting either a Movie
// or an Episode. The ID provided is used to lookup both, and whichever
// query is successful is used to populate a media Container.
func (store *Store) GetMedia(db *gorm.DB, mediaId uuid.UUID) *Container {
	if movie, err := store.GetMovie(db, mediaId); err != nil {
		if episode, err := store.GetEpisode(db, mediaId); err != nil {
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
func (store *Store) GetMovie(db *gorm.DB, movieId uuid.UUID) (*Movie, error) {
	var result Movie
	err := db.Where(&Movie{Common: Common{Id: movieId}}).First(&result).Error
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// GetEpisode will search the database for a Episode row matching the
// PK ID provided. No result will cause 'nil' to be returned, failure
// for any other reason will see 'nil' returned.
func (store *Store) GetEpisode(db *gorm.DB, episodeId uuid.UUID) (*Episode, error) {
	var result Episode
	err := db.Where(&Episode{Common: Common{Id: episodeId}}).First(&result).Error
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// GetSeason will search the database for a Season row matching the
// PK ID provided. No result will cause 'nil' to be returned, failure
// for any other reason will see 'nil' returned.
func (store *Store) GetSeason(db *gorm.DB, seasonId uuid.UUID) (*Season, error) {
	var result Season
	err := db.Where(&Season{Id: seasonId}).First(&result).Error
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// GetSeries will search the database for a Series row matching the
// PK ID provided. No result will cause 'nil' to be returned, failure
// for any other reason will see 'nil' returned.
func (store *Store) GetSeries(db *gorm.DB, seriesId uuid.UUID) (*Series, error) {
	var result Series
	err := db.Where(&Series{Id: seriesId}).First(&result).Error
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// GetAllSourcePaths returns all the source paths related
// to media that is currently known to Thea by polling the database.
func (store *Store) GetAllSourcePaths(db *gorm.DB) []string {
	return make([]string, 0)
}
