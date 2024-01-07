package media

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type (
	// Container is a struct which contains either a Movie or
	// an Episode. This is indicated using the 'Type' enum. If
	// container is holding an 'Episode' type, then the 'Season'
	// and 'Series' that the episode belongs to will also be populated
	// if available
	ContainerType int
	Container     struct {
		Type    ContainerType
		Movie   *Movie
		Episode *Episode
		Series  *Series
		Season  *Season
	}
)

const (
	MOVIE ContainerType = iota
	EPISODE
	SERIES
)

func (cont *Container) Resolution() (int, int) { return 0, 0 }
func (cont *Container) Id() uuid.UUID          { return cont.model().ID }
func (cont *Container) Title() string          { return cont.model().Title }
func (cont *Container) TmdbId() string         { return cont.model().TmdbID }
func (cont *Container) CreatedAt() time.Time   { return cont.model().CreatedAt }
func (cont *Container) UpdatedAt() time.Time   { return cont.model().UpdatedAt }
func (cont *Container) Source() string         { return cont.watchable().SourcePath }

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

	return cont.Season.SeasonNumber
}

func (cont *Container) String() string {
	return fmt.Sprintf("{media title=%s | id=%s | tmdb_id=%s }", cont.model().Title, cont.model().ID, cont.model().TmdbID)
}

func (cont *Container) watchable() *Watchable {
	switch cont.Type {
	case MOVIE:
		return &cont.Movie.Watchable
	case EPISODE:
		return &cont.Episode.Watchable
	default:
		panic("Cannot fetch watchable from container due to unknown container type")
	}
}

func (cont *Container) model() *Model {
	switch cont.Type {
	case MOVIE:
		return &cont.Movie.Model
	case EPISODE:
		return &cont.Episode.Model
	case SERIES:
		return &cont.Series.Model
	default:
		panic("Cannot fetch model from container due to unknown container type")
	}
}
