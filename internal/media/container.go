package media

import (
	"fmt"

	"github.com/google/uuid"
)

type (
	// Container is a struct which contains either a Movie or
	// an Episode. This is indicated using the 'Type' enum.
	ContainerType int
	Container     struct {
		Type    ContainerType
		Movie   *Movie
		Episode *Episode
	}

	// Stub represents the minimal information required to represent
	// a partials search result entry from a media searcher. This information
	// is mainly used when a searcher encounters multiple results and needs to
	// prompt the user to select the correct one.
	Stub struct {
		Type       ContainerType
		PosterPath string
		Title      string
		SourceID   string
	}
)

const (
	MOVIE ContainerType = iota
	EPISODE
)

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
