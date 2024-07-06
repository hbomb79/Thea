package match_test

import (
	"slices"
	"testing"

	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/internal/workflow/match"
	"github.com/stretchr/testify/assert"
)

type criteriaTest struct {
	summary   string
	criteria  match.Criteria
	isValid   bool
	shouldErr bool
}

func runCriteriaValidationTests(t *testing.T, tests []criteriaTest) {
	for _, tt := range tests {
		t.Run(tt.summary, func(t *testing.T) {
			err := tt.criteria.ValidateLegal()
			if tt.shouldErr {
				assert.Error(t, err, "ValidateLegal() expected to return an error")
			} else {
				assert.NoError(t, err, "ValidateLegal() returned an error when it was not expected")
			}
		})
	}
}

func Test_ValidateLegal(t *testing.T) {
	strTypes := []match.Type{
		match.Matches,
		match.DoesNotMatch,
	}
	strKeys := []match.Key{
		match.MediaTitleKey,
		match.SeriesTitleKey,
		match.SeasonTitleKey,
		match.ResolutionKey,
		match.SourcePathKey,
		match.SourceNameKey,
		match.SourceExtensionKey,
	}

	numTypes := []match.Type{
		match.Equals,
		match.NotEquals,
		match.LessThan,
		match.GreaterThan,
	}
	numKeys := []match.Key{
		match.EpisodeNumberKey,
		match.SeasonNumberKey,
	}

	runTests := func(summary string, types []match.Type, keys []match.Key, value string, isValid bool, shouldErr bool) {
		tests := make([]criteriaTest, 0, len(keys)*len(types))
		for _, typ := range types {
			for _, k := range keys {
				tests = append(tests, criteriaTest{
					summary:   typ.String() + "/" + k.String(),
					criteria:  match.Criteria{Key: k, Type: typ, Value: value},
					isValid:   isValid,
					shouldErr: shouldErr,
				})
			}
		}

		t.Run(summary, func(t *testing.T) {
			runCriteriaValidationTests(t, tests)
		})
	}

	// Success
	runTests("String OK", strTypes, strKeys, "Valid string", true, false)
	runTests("String Regex OK", strTypes, strKeys, "/^valid re.+$/", true, false)
	runTests("Number OK", numTypes, numKeys, "4", true, false)

	// Failure
	runTests("String Regex Invalid", strTypes, strKeys, "/.++/", false, true)
	runTests("String Empty", strTypes, strKeys, "", false, true)
	runTests("Number Invalid", numTypes, numKeys, "notanumber", false, true)
	runTests("Number Not Int", numTypes, numKeys, "-0.4", false, true)
}

func Test_ValidateLegal_AcceptableType(t *testing.T) {
	runTests := func(t *testing.T, typeToTest match.Type, value string, validKeys []match.Key) {
		tests := make([]criteriaTest, 0)
		for _, k := range []match.Key{
			match.MediaTitleKey, match.SeriesTitleKey, match.SeasonTitleKey,
			match.ResolutionKey, match.SeasonNumberKey, match.EpisodeNumberKey,
			match.SourcePathKey, match.SourceNameKey, match.SourceExtensionKey,
		} {
			tests = append(tests, criteriaTest{
				summary:   k.String(),
				criteria:  match.Criteria{Key: k, Type: typeToTest, Value: value},
				shouldErr: !slices.Contains(validKeys, k),
			})
		}

		t.Run(typeToTest.String(), func(t *testing.T) {
			runCriteriaValidationTests(t, tests)
		})
	}

	runTests(t, match.Equals, "0", []match.Key{
		match.SeasonNumberKey,
		match.EpisodeNumberKey,
	})
	runTests(t, match.NotEquals, "0", []match.Key{
		match.SeasonNumberKey,
		match.EpisodeNumberKey,
	})
	runTests(t, match.Matches, "str", []match.Key{
		match.MediaTitleKey,
		match.SeriesTitleKey,
		match.SeasonTitleKey,
		match.ResolutionKey,
		match.SourcePathKey,
		match.SourceNameKey,
		match.SourceExtensionKey,
	})
	runTests(t, match.DoesNotMatch, "str", []match.Key{
		match.MediaTitleKey,
		match.SeriesTitleKey,
		match.SeasonTitleKey,
		match.ResolutionKey,
		match.SourcePathKey,
		match.SourceNameKey,
		match.SourceExtensionKey,
	})
	runTests(t, match.LessThan, "0", []match.Key{
		match.SeasonNumberKey,
		match.EpisodeNumberKey,
	})
	runTests(t, match.GreaterThan, "0", []match.Key{
		match.SeasonNumberKey,
		match.EpisodeNumberKey,
	})
	runTests(t, match.IsPresent, "true", []match.Key{
		match.MediaTitleKey,
		match.SeriesTitleKey,
		match.SeasonTitleKey,
		match.ResolutionKey,
		match.SeasonNumberKey,
		match.EpisodeNumberKey,
		match.SourcePathKey,
		match.SourceNameKey,
		match.SourceExtensionKey,
	})
	runTests(t, match.IsNotPresent, "true", []match.Key{
		match.MediaTitleKey,
		match.SeriesTitleKey,
		match.SeasonTitleKey,
		match.ResolutionKey,
		match.SeasonNumberKey,
		match.EpisodeNumberKey,
		match.SourcePathKey,
		match.SourceNameKey,
		match.SourceExtensionKey,
	})
}

func runMediaAcceptableTests(t *testing.T, media *media.Container, tests []criteriaTest) {
	for _, tt := range tests {
		t.Run(tt.summary, func(t *testing.T) {
			ok, err := tt.criteria.IsMediaAcceptable(media)

			if tt.isValid {
				assert.Truef(t, ok, "IsMediaAcceptable(%v) expected to return true", media)
			} else {
				assert.Falsef(t, ok, "IsMediaAcceptable(%v) expected to return false", media)
			}

			if tt.shouldErr {
				assert.Error(t, err, "IsMediaAcceptable(%v) expected to return an error", media)
			} else {
				assert.NoError(t, err, "IsMediaAcceptable(%v) returned an error when it was not expected", media)
			}
		})
	}
}

//nolint:dupl,funlen
func Test_MovieAcceptable(t *testing.T) {
	movie := &media.Container{
		Type: media.MovieContainerType,
		Movie: &media.Movie{
			Model: media.Model{Title: "Example Movie"},
			Watchable: media.Watchable{
				MediaResolution: media.MediaResolution{Width: 1920, Height: 1080},
				SourcePath:      "/home/foo/source/media.mp4",
				Adult:           true,
			},
			Genres: []*media.Genre{
				{Label: "Action"},
				{Label: "Drama"},
			},
		},
	}

	runCommonMediaAcceptableTests(t, movie)

	t.Run("Title", func(t *testing.T) {
		tests := []criteriaTest{
			{
				summary:   "Is Present",
				criteria:  match.Criteria{Key: match.MediaTitleKey, Type: match.IsPresent, Value: ""},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Not Present",
				criteria:  match.Criteria{Key: match.MediaTitleKey, Type: match.IsNotPresent, Value: ""},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive string match",
				criteria:  match.Criteria{Key: match.MediaTitleKey, Type: match.Matches, Value: "Example Movie"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative string match",
				criteria:  match.Criteria{Key: match.MediaTitleKey, Type: match.Matches, Value: "An Example Movie"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive string regexp match",
				criteria:  match.Criteria{Key: match.MediaTitleKey, Type: match.Matches, Value: "/Movie/"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative string regexp match",
				criteria:  match.Criteria{Key: match.MediaTitleKey, Type: match.Matches, Value: "/Foo/"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Invalid match type",
				criteria:  match.Criteria{Key: match.MediaTitleKey, Type: match.Equals, Value: "Example Movie"},
				isValid:   false,
				shouldErr: true,
			},
		}

		runMediaAcceptableTests(t, movie, tests)
	})

	t.Run("Episodic keys", func(t *testing.T) {
		tests := []criteriaTest{
			{
				summary:   "Series title IsPresent",
				criteria:  match.Criteria{Key: match.SeriesTitleKey, Type: match.IsPresent, Value: ""},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Series title IsNotPresent",
				criteria:  match.Criteria{Key: match.SeriesTitleKey, Type: match.IsNotPresent, Value: ""},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Season title IsPresent",
				criteria:  match.Criteria{Key: match.SeasonTitleKey, Type: match.IsPresent, Value: ""},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Season title IsNotPresent",
				criteria:  match.Criteria{Key: match.SeasonTitleKey, Type: match.IsNotPresent, Value: ""},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Season number IsPresent",
				criteria:  match.Criteria{Key: match.SeasonNumberKey, Type: match.IsPresent, Value: ""},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Season number IsNotPresent",
				criteria:  match.Criteria{Key: match.SeasonNumberKey, Type: match.IsNotPresent, Value: ""},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Episode number IsPresent",
				criteria:  match.Criteria{Key: match.EpisodeNumberKey, Type: match.IsPresent, Value: ""},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Episode number IsNotPresent",
				criteria:  match.Criteria{Key: match.EpisodeNumberKey, Type: match.IsNotPresent, Value: ""},
				isValid:   true,
				shouldErr: false,
			},
		}

		runMediaAcceptableTests(t, movie, tests)
	})
}

//nolint:funlen,dupl,maintidx
func Test_EpisodeAcceptable(t *testing.T) {
	episode := &media.Container{
		Type: media.EpisodeContainerType,
		Episode: &media.Episode{
			Model: media.Model{Title: "Example Episode"},
			Watchable: media.Watchable{
				MediaResolution: media.MediaResolution{Width: 1920, Height: 1080},
				SourcePath:      "/home/foo/source/media.mp4",
				Adult:           true,
			},
			EpisodeNumber: 5,
		},
		Season: &media.Season{
			Model:        media.Model{Title: "Example Season"},
			SeasonNumber: 2,
		},
		Series: &media.Series{
			Model: media.Model{Title: "Example Series"},
			Genres: []*media.Genre{
				{Label: "Action"},
				{Label: "Drama"},
			},
		},
	}

	runCommonMediaAcceptableTests(t, episode)

	t.Run("MediaTitle", func(t *testing.T) {
		tests := []criteriaTest{
			{
				summary:   "Is Present",
				criteria:  match.Criteria{Key: match.MediaTitleKey, Type: match.IsPresent, Value: ""},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Not Present",
				criteria:  match.Criteria{Key: match.MediaTitleKey, Type: match.IsNotPresent, Value: ""},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive string match",
				criteria:  match.Criteria{Key: match.MediaTitleKey, Type: match.Matches, Value: "Example Episode"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative string match",
				criteria:  match.Criteria{Key: match.MediaTitleKey, Type: match.Matches, Value: "An Example Episode"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive string regexp match",
				criteria:  match.Criteria{Key: match.MediaTitleKey, Type: match.Matches, Value: "/Episode/"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative string regexp match",
				criteria:  match.Criteria{Key: match.MediaTitleKey, Type: match.Matches, Value: "/Foo/"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Invalid match type",
				criteria:  match.Criteria{Key: match.MediaTitleKey, Type: match.Equals, Value: "Example Episode"},
				isValid:   false,
				shouldErr: true,
			},
		}

		runMediaAcceptableTests(t, episode, tests)
	})

	t.Run("EpisodeNumber", func(t *testing.T) {
		tests := []criteriaTest{
			{
				summary:   "IsPresent",
				criteria:  match.Criteria{Key: match.EpisodeNumberKey, Type: match.IsPresent, Value: ""},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "IsNotPresent",
				criteria:  match.Criteria{Key: match.EpisodeNumberKey, Type: match.IsNotPresent, Value: ""},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive Equals",
				criteria:  match.Criteria{Key: match.EpisodeNumberKey, Type: match.Equals, Value: "5"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative Equals",
				criteria:  match.Criteria{Key: match.EpisodeNumberKey, Type: match.Equals, Value: "22"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive LessThan",
				criteria:  match.Criteria{Key: match.EpisodeNumberKey, Type: match.LessThan, Value: "1"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative LessThan",
				criteria:  match.Criteria{Key: match.EpisodeNumberKey, Type: match.LessThan, Value: "7"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive GreaterThan",
				criteria:  match.Criteria{Key: match.EpisodeNumberKey, Type: match.GreaterThan, Value: "7"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative GreaterThan",
				criteria:  match.Criteria{Key: match.EpisodeNumberKey, Type: match.GreaterThan, Value: "1"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Invalid match type",
				criteria:  match.Criteria{Key: match.EpisodeNumberKey, Type: match.Matches, Value: "Example"},
				isValid:   false,
				shouldErr: true,
			},
		}

		runMediaAcceptableTests(t, episode, tests)
	})

	t.Run("SeasonTitle", func(t *testing.T) {
		tests := []criteriaTest{
			{
				summary:   "Is Present",
				criteria:  match.Criteria{Key: match.SeasonTitleKey, Type: match.IsPresent, Value: ""},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Not Present",
				criteria:  match.Criteria{Key: match.SeasonTitleKey, Type: match.IsNotPresent, Value: ""},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive string match",
				criteria:  match.Criteria{Key: match.SeasonTitleKey, Type: match.Matches, Value: "Example Season"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative string match",
				criteria:  match.Criteria{Key: match.SeasonTitleKey, Type: match.Matches, Value: "An Example Season"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive string regexp match",
				criteria:  match.Criteria{Key: match.SeasonTitleKey, Type: match.Matches, Value: "/Season/"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative string regexp match",
				criteria:  match.Criteria{Key: match.SeasonTitleKey, Type: match.Matches, Value: "/Foo/"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Invalid match type",
				criteria:  match.Criteria{Key: match.SeasonTitleKey, Type: match.Equals, Value: "Example Season"},
				isValid:   false,
				shouldErr: true,
			},
		}

		runMediaAcceptableTests(t, episode, tests)
	})

	t.Run("SeasonNumber", func(t *testing.T) {
		tests := []criteriaTest{
			{
				summary:   "IsPresent",
				criteria:  match.Criteria{Key: match.SeasonNumberKey, Type: match.IsPresent, Value: ""},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "IsNotPresent",
				criteria:  match.Criteria{Key: match.SeasonNumberKey, Type: match.IsNotPresent, Value: ""},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive Equals",
				criteria:  match.Criteria{Key: match.SeasonNumberKey, Type: match.Equals, Value: "2"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative Equals",
				criteria:  match.Criteria{Key: match.SeasonNumberKey, Type: match.Equals, Value: "22"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive LessThan",
				criteria:  match.Criteria{Key: match.SeasonNumberKey, Type: match.LessThan, Value: "1"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative LessThan",
				criteria:  match.Criteria{Key: match.SeasonNumberKey, Type: match.LessThan, Value: "3"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive GreaterThan",
				criteria:  match.Criteria{Key: match.SeasonNumberKey, Type: match.GreaterThan, Value: "10"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative GreaterThan",
				criteria:  match.Criteria{Key: match.SeasonNumberKey, Type: match.GreaterThan, Value: "1"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Invalid match type",
				criteria:  match.Criteria{Key: match.SeasonNumberKey, Type: match.Matches, Value: "Example Season"},
				isValid:   false,
				shouldErr: true,
			},
		}

		runMediaAcceptableTests(t, episode, tests)
	})

	t.Run("SeriesTitle", func(t *testing.T) {
		tests := []criteriaTest{
			{
				summary:   "Is Present",
				criteria:  match.Criteria{Key: match.SeriesTitleKey, Type: match.IsPresent, Value: ""},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Not Present",
				criteria:  match.Criteria{Key: match.SeriesTitleKey, Type: match.IsNotPresent, Value: ""},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive string match",
				criteria:  match.Criteria{Key: match.SeriesTitleKey, Type: match.Matches, Value: "Example Series"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative string match",
				criteria:  match.Criteria{Key: match.SeriesTitleKey, Type: match.Matches, Value: "An Example Series"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive string regexp match",
				criteria:  match.Criteria{Key: match.SeriesTitleKey, Type: match.Matches, Value: "/Series/"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative string regexp match",
				criteria:  match.Criteria{Key: match.SeriesTitleKey, Type: match.Matches, Value: "/Foo/"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Invalid match type",
				criteria:  match.Criteria{Key: match.SeriesTitleKey, Type: match.Equals, Value: "Example Series"},
				isValid:   false,
				shouldErr: true,
			},
		}

		runMediaAcceptableTests(t, episode, tests)
	})
}

// runCommonMediaAcceptableTests contains the common set
// of tests for validating media which applies to both movies
// and episodes.
//
//nolint:dupl,funlen
func runCommonMediaAcceptableTests(t *testing.T, media *media.Container) {
	t.Run("Resolution", func(t *testing.T) {
		tests := []criteriaTest{
			{
				summary:   "Is Present",
				criteria:  match.Criteria{Key: match.ResolutionKey, Type: match.IsPresent, Value: ""},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Not Present",
				criteria:  match.Criteria{Key: match.ResolutionKey, Type: match.IsNotPresent, Value: ""},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive string match",
				criteria:  match.Criteria{Key: match.ResolutionKey, Type: match.Matches, Value: "1920x1080"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative string match",
				criteria:  match.Criteria{Key: match.ResolutionKey, Type: match.Matches, Value: "1080x720"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive string regexp match",
				criteria:  match.Criteria{Key: match.ResolutionKey, Type: match.Matches, Value: "/1920x/"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative string regexp match",
				criteria:  match.Criteria{Key: match.ResolutionKey, Type: match.Matches, Value: "/720/"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Invalid match type",
				criteria:  match.Criteria{Key: match.ResolutionKey, Type: match.Equals, Value: "1920x1080"},
				isValid:   false,
				shouldErr: true,
			},
		}

		runMediaAcceptableTests(t, media, tests)
	})

	t.Run("SourcePath", func(t *testing.T) {
		tests := []criteriaTest{
			{
				summary:   "Is Present",
				criteria:  match.Criteria{Key: match.SourcePathKey, Type: match.IsPresent, Value: ""},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Not Present",
				criteria:  match.Criteria{Key: match.SourcePathKey, Type: match.IsNotPresent, Value: ""},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive string match",
				criteria:  match.Criteria{Key: match.SourcePathKey, Type: match.Matches, Value: "/home/foo/source/media.mp4"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative string match",
				criteria:  match.Criteria{Key: match.SourcePathKey, Type: match.Matches, Value: "/home/foo/source/media.mkv"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive string regexp match",
				criteria:  match.Criteria{Key: match.SourcePathKey, Type: match.Matches, Value: "/media.*/"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative string regexp match",
				criteria:  match.Criteria{Key: match.SourcePathKey, Type: match.Matches, Value: "/media.mkv/"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Invalid match type",
				criteria:  match.Criteria{Key: match.SourcePathKey, Type: match.Equals, Value: "/home/foo/source/media.mp4"},
				isValid:   false,
				shouldErr: true,
			},
		}

		runMediaAcceptableTests(t, media, tests)
	})

	t.Run("SourceExtension", func(t *testing.T) {
		tests := []criteriaTest{
			{
				summary:   "Is Present",
				criteria:  match.Criteria{Key: match.SourceExtensionKey, Type: match.IsPresent, Value: ""},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Not Present",
				criteria:  match.Criteria{Key: match.SourceExtensionKey, Type: match.IsNotPresent, Value: ""},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive string match",
				criteria:  match.Criteria{Key: match.SourceExtensionKey, Type: match.Matches, Value: ".mp4"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative string match",
				criteria:  match.Criteria{Key: match.SourceExtensionKey, Type: match.Matches, Value: ".mkv"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive string regexp match",
				criteria:  match.Criteria{Key: match.SourceExtensionKey, Type: match.Matches, Value: "/\\.mp*/"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative string regexp match",
				criteria:  match.Criteria{Key: match.SourceExtensionKey, Type: match.Matches, Value: "/\\.mk./"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Invalid match type",
				criteria:  match.Criteria{Key: match.SourceExtensionKey, Type: match.Equals, Value: ".mp4"},
				isValid:   false,
				shouldErr: true,
			},
		}

		runMediaAcceptableTests(t, media, tests)
	})

	t.Run("SourceName", func(t *testing.T) {
		tests := []criteriaTest{
			{
				summary:   "Is Present",
				criteria:  match.Criteria{Key: match.SourceNameKey, Type: match.IsPresent, Value: ""},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Not Present",
				criteria:  match.Criteria{Key: match.SourceNameKey, Type: match.IsNotPresent, Value: ""},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive string match",
				criteria:  match.Criteria{Key: match.SourceNameKey, Type: match.Matches, Value: "media.mp4"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative string match",
				criteria:  match.Criteria{Key: match.SourceNameKey, Type: match.Matches, Value: "media.mkv"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Positive string regexp match",
				criteria:  match.Criteria{Key: match.SourceNameKey, Type: match.Matches, Value: "/media.*/"},
				isValid:   true,
				shouldErr: false,
			},
			{
				summary:   "Negative string regexp match",
				criteria:  match.Criteria{Key: match.SourceNameKey, Type: match.Matches, Value: "/media.mkv/"},
				isValid:   false,
				shouldErr: false,
			},
			{
				summary:   "Invalid match type",
				criteria:  match.Criteria{Key: match.SourceNameKey, Type: match.Equals, Value: "media.mp4"},
				isValid:   false,
				shouldErr: true,
			},
		}

		runMediaAcceptableTests(t, media, tests)
	})
}
