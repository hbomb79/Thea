package media

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type (
	ContainerType int

	// Container is a struct which contains either a Movie or
	// an Episode. This is indicated using the 'Type' enum. If
	// container is holding an 'Episode' type, then the 'Season'
	// and 'Series' that the episode belongs to will also be populated
	// if available
	Container struct {
		Type    ContainerType
		Movie   *Movie
		Episode *Episode
		Series  *Series
		Season  *Season
	}
)

const (
	MovieContainerType ContainerType = iota
	EpisodeContainerType
	SeriesContainerType
)

func (cont *Container) Resolution() (int, int) { return 0, 0 }
func (cont *Container) ID() uuid.UUID          { return cont.model().ID }
func (cont *Container) Title() string          { return cont.model().Title }
func (cont *Container) TmdbID() string         { return cont.model().TmdbID }
func (cont *Container) CreatedAt() time.Time   { return cont.model().CreatedAt }
func (cont *Container) UpdatedAt() time.Time   { return cont.model().UpdatedAt }
func (cont *Container) Source() string         { return cont.watchable().SourcePath }

// EpisodeNumber returns the episode number for the media IF it is an Episode. -1
// is returned if the container is holding a Movie.
func (cont *Container) EpisodeNumber() int {
	if cont.Type == MovieContainerType {
		return -1
	}

	return cont.Episode.EpisodeNumber
}

// SeasonNumber returns the season number for the media IF it is an Episode. -1
// is returned if the container is holding a Movie.
func (cont *Container) SeasonNumber() int {
	if cont.Type == MovieContainerType {
		return -1
	}

	return cont.Season.SeasonNumber
}

func (cont *Container) String() string {
	return fmt.Sprintf("{media title=%s | id=%s | tmdb_id=%s }", cont.model().Title, cont.model().ID, cont.model().TmdbID)
}

func (cont *Container) watchable() *Watchable {
	switch cont.Type {
	case MovieContainerType:
		return &cont.Movie.Watchable
	case EpisodeContainerType:
		return &cont.Episode.Watchable
	default:
		panic("Cannot fetch watchable from container due to unknown container type")
	}
}

func (cont *Container) model() *Model {
	switch cont.Type {
	case MovieContainerType:
		return &cont.Movie.Model
	case EpisodeContainerType:
		return &cont.Episode.Model
	case SeriesContainerType:
		return &cont.Series.Model
	default:
		panic("Cannot fetch model from container due to unknown container type")
	}
}
