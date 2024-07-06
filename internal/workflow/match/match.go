package match

type Key int

const (
	// MediaTitleKey will match against the title of
	// the media directly. Either the episode title (for
	// episode types), or the movie title (for movie types).
	MediaTitleKey Key = iota

	// SeriesTitleKey matches against the title of the series
	// that the container contains. This will of course fail
	// if the media being matched against is a movie.
	SeriesTitleKey

	// SeasonTitleKey matches against the title of the season that
	// the container contains. This will fail if the media being
	// matched against is a movie.
	SeasonTitleKey

	ResolutionKey
	SeasonNumberKey
	EpisodeNumberKey
	SourcePathKey
	SourceNameKey
	SourceExtensionKey
)

func (e Key) Values() []string {
	return []string{
		"MEDIA_TITLE", "SERIES_TITLE", "SEASON_TITLE",
		"RESOLUTION", "SEASON_NUMBER", "EPISODE_NUMBER",
		"SOURCE_PATH", "SOURCE_NAME", "SOURCE_EXTENSION",
	}
}

func (e Key) String() string {
	return e.Values()[e]
}

type Type int

const (
	Equals Type = iota
	NotEquals
	Matches
	DoesNotMatch
	LessThan
	GreaterThan
	IsPresent
	IsNotPresent
)

func IsTypeAcceptable(key Key, t Type) bool {
	acceptableTypes := keyAcceptableTypes()
	if matchTypes, ok := acceptableTypes[key]; ok {
		for _, v := range matchTypes {
			if v == t {
				return true
			}
		}
	}

	return false
}

func keyAcceptableTypes() map[Key][]Type {
	return map[Key][]Type{
		MediaTitleKey:      {Matches, DoesNotMatch, IsNotPresent, IsPresent},
		SeasonTitleKey:     {Matches, DoesNotMatch, IsNotPresent, IsPresent},
		SeriesTitleKey:     {Matches, DoesNotMatch, IsNotPresent, IsPresent},
		ResolutionKey:      {Matches, DoesNotMatch, IsNotPresent, IsPresent},
		SeasonNumberKey:    {Equals, NotEquals, LessThan, GreaterThan, IsNotPresent, IsPresent},
		EpisodeNumberKey:   {Equals, NotEquals, LessThan, GreaterThan, IsNotPresent, IsPresent},
		SourcePathKey:      {Matches, DoesNotMatch, IsPresent, IsNotPresent},
		SourceNameKey:      {Matches, DoesNotMatch, IsPresent, IsNotPresent},
		SourceExtensionKey: {Matches, DoesNotMatch, IsPresent, IsNotPresent},
	}
}

func (e Type) Values() []string {
	return []string{"EQUALS", "NOT_EQUALS", "MATCHES", "DOES_NOT_MATCH", "LESS_THAN", "GREATER_THAN", "IS_PRESENT", "IS_NOT_PRESENT"}
}

func (e Type) String() string {
	return e.Values()[e]
}

type CombineType int

const (
	AND CombineType = iota
	OR
)

func (e CombineType) Values() []string {
	return []string{"AND", "OR"}
}

func (e CombineType) String() string {
	return e.Values()[e]
}
