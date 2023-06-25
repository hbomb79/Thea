package media

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Season represents the information Thea stores about a season
// of episodes itself. A season is related to many episodes (however
// this payload does not contain them); additionally, a series is related
// to many seasons.
type Season struct {
	Id        uuid.UUID `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	TmdbId    string `gorm:"uniqueIndex"`
}

// Series represents the information Thea stores about a series. A one-to-many
// relationship exists between series and seasons, although the seasons themselves
// are not contained within this struct.
type Series struct {
	Id        uuid.UUID `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	TmdbId    string `gorm:"uniqueIndex"`
}

// Episode contains all the information unique to an episode, combined
// with the 'Common' struct.
type Episode struct {
	SeasonNumber  int
	EpisodeNumber int
	Common
}

type Movie struct {
	Common
}

type Common struct {
	Id         uuid.UUID `gorm:"primaryKey"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Title      string
	Resolution int
	SourcePath string
	TmdbId     string `gorm:"uniqueIndex"`
}

type ContainerType int

const (
	MOVIE ContainerType = iota
	EPISODE
)

// Container is a struct which contains either a Movie or
// an Episode. This is indicated using the 'Type' enum.
type Container struct {
	Type    ContainerType
	Movie   *Movie
	Episode *Episode
}

func (cont *Container) Resolution() (int, int) { return 0, 0 }
func (cont *Container) Id() uuid.UUID          { return cont.common().Id }
func (cont *Container) Title() string          { return cont.common().Title }
func (cont *Container) TmdbId() string         { return cont.common().TmdbId }
func (cont *Container) Source() string         { return cont.common().SourcePath }

// EpisodeNumber returns the episode number for the media IF it is an Episode. -1
// is returned if the container is holding a Movie.
func (cont *Container) EpisodeNumber() int {
	if cont.Type == MOVIE {
		return -1
	}

	return cont.Episode.EpisodeNumber
}

// SeasonNumber returns the season number for the media IF it is an Episode. -1
// is returned if the container is holding a Movie.
func (cont *Container) SeasonNumber() int {
	if cont.Type == MOVIE {
		return -1
	}

	return cont.Episode.SeasonNumber
}

func (cont *Container) String() string {
	return fmt.Sprintf("{media title=%s | id=%s | tmdb_id=%s }", cont.common().Title, cont.common().Id, cont.common().TmdbId)
}

func (cont *Container) common() *Common {
	switch cont.Type {
	case MOVIE:
		return &cont.Movie.Common
	case EPISODE:
		return &cont.Episode.Common
	default:
		panic("Container type unknown?")
	}
}

// SearchStub represents the minimal information required to represent
// a partials search result entry from a media searcher. This information
// is mainly used when a searcher encounters multiple results and needs to
// prompt the user to select the correct one.
type SearchStub struct {
	Type       ContainerType
	PosterPath string
	Title      string
	SourceID   string
}
