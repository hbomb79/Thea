package media

import "gorm.io/gorm"

// Season represents the information Thea stores about a season
// of episodes itself. A season is related to many episodes (however
// this payload does not contain them); additionally, a series is related
// to many seasons.
type Season struct {
	gorm.Model
}

// Series represents the information Thea stores about a series. A one-t-many
// relationship exists between series and seasons, although the seasons themselves
// are not contained within this struct.
type Series struct {
	gorm.Model
}

// Episode contains all the information unique to an episode, combined
// with the 'Common' struct.
type Episode struct {
	gorm.Model
}

type Movie struct {
	gorm.Model
}

type ContainerType int

const (
	MOVIE ContainerType = iota
	EPISODE
)

// SearchResult is a struct which can contain EITHER
// a movie or an episode struct pointer.
// This is indicated using the 'Type' enum
type MediaContainer struct {
	Type    ContainerType
	Movie   *Movie
	Episode *Episode
}

func (cont *MediaContainer) Source() string {
	return ""
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
