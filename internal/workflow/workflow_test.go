package workflow_test

import (
	"testing"

	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/internal/workflow"
	"github.com/hbomb79/Thea/internal/workflow/match"
	"github.com/labstack/gommon/random"
	"github.com/stretchr/testify/assert"
)

type workflowTest struct {
	summary    string
	workflow   workflow.Workflow
	isEligible bool
}

func runWorkflowTests(t *testing.T, container *media.Container, tests []workflowTest) {
	for _, tt := range tests {
		t.Run(tt.summary, func(t *testing.T) {
			ok := tt.workflow.IsMediaEligible(container)
			if tt.isEligible {
				assert.Truef(t, ok, "IsMediaEligible(%v) expected to return true", container)
			} else {
				assert.Falsef(t, ok, "IsMediaEligible(%v) expected to return false", container)
			}
		})
	}
}

func createEmptyWorkflow(criteria []match.Criteria) workflow.Workflow {
	return workflow.Workflow{Enabled: true, Label: random.String(10, random.Alphanumeric), Criteria: criteria}
}

func Test_Workflow_MovieEligible(t *testing.T) {
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

	runCommonMediaWorkflowTests(t, movie)
}

func Test_Workflow_EpisodeEligible(t *testing.T) {
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

	runCommonMediaWorkflowTests(t, episode)
}

// runCommonMediaWorkflowTests runs a set of workflowTests which should
// be valid for any type of media (movie or episode).
func runCommonMediaWorkflowTests(t *testing.T, container *media.Container) {
	tests := []workflowTest{
		{
			summary:    "empty",
			workflow:   createEmptyWorkflow([]match.Criteria{}),
			isEligible: true,
		},
		{
			summary:    "true",
			workflow:   createEmptyWorkflow([]match.Criteria{{Key: match.MediaTitleKey, Type: match.IsPresent}}),
			isEligible: true,
		},
		{
			summary:    "false",
			workflow:   createEmptyWorkflow([]match.Criteria{{Key: match.SourceExtensionKey, Type: match.Matches, Value: "mp3"}}),
			isEligible: false,
		},
		{
			summary: "true & true",
			workflow: createEmptyWorkflow([]match.Criteria{
				{Key: match.SourceExtensionKey, Type: match.Matches, Value: ".mp4", CombineType: match.AND},
				{Key: match.MediaTitleKey, Type: match.Matches, Value: "/Example/"},
			}),
			isEligible: true,
		},
		{
			summary: "false || true",
			workflow: createEmptyWorkflow([]match.Criteria{
				{Key: match.SourceExtensionKey, Type: match.Matches, Value: ".mkv", CombineType: match.OR},
				{Key: match.SourcePathKey, Type: match.Matches, Value: "/source/media/"},
			}),
			isEligible: true,
		},
		{
			summary: "(true && true) || false",
			workflow: createEmptyWorkflow([]match.Criteria{
				{Key: match.SourceExtensionKey, Type: match.Matches, Value: ".mp4", CombineType: match.AND},
				{Key: match.ResolutionKey, Type: match.Matches, Value: "1920x1080", CombineType: match.OR},
				{Key: match.SourcePathKey, Type: match.Matches, Value: "suishfsjaf"},
			}),
			isEligible: true,
		},
		{
			summary: "(false && false) || true",
			workflow: createEmptyWorkflow([]match.Criteria{
				{Key: match.SourceExtensionKey, Type: match.Matches, Value: ".mkv", CombineType: match.AND},
				{Key: match.ResolutionKey, Type: match.Matches, Value: "720", CombineType: match.OR},
				{Key: match.SourcePathKey, Type: match.Matches, Value: "/source/media/"},
			}),
			isEligible: true,
		},
		{
			summary: "(false && false) || false",
			workflow: createEmptyWorkflow([]match.Criteria{
				{Key: match.SourceExtensionKey, Type: match.Matches, Value: ".mkv", CombineType: match.AND},
				{Key: match.ResolutionKey, Type: match.Matches, Value: "720", CombineType: match.OR},
				{Key: match.SourcePathKey, Type: match.Matches, Value: "/sauce/media/"},
			}),
			isEligible: false,
		},
		{
			summary: "(false && false) || false || (true && false)",
			workflow: createEmptyWorkflow([]match.Criteria{
				{Key: match.SourceExtensionKey, Type: match.Matches, Value: ".mkv", CombineType: match.AND},
				{Key: match.ResolutionKey, Type: match.Matches, Value: "720", CombineType: match.OR},
				{Key: match.SourcePathKey, Type: match.Matches, Value: "/baz/", CombineType: match.OR},
				{Key: match.SourceExtensionKey, Type: match.Matches, Value: ".mp4", CombineType: match.AND},
				{Key: match.MediaTitleKey, Type: match.Matches, Value: "NotATitle"},
			}),
			isEligible: false,
		},
		{
			summary: "(false && false) || false || true && false || true",
			workflow: createEmptyWorkflow([]match.Criteria{
				{Key: match.SourceExtensionKey, Type: match.Matches, Value: ".mkv", CombineType: match.AND},
				{Key: match.ResolutionKey, Type: match.Matches, Value: "720", CombineType: match.OR},
				{Key: match.SourcePathKey, Type: match.Matches, Value: "/baz/", CombineType: match.OR},
				{Key: match.SourceExtensionKey, Type: match.Matches, Value: ".mp4", CombineType: match.AND},
				{Key: match.MediaTitleKey, Type: match.Matches, Value: "NotATitle", CombineType: match.OR},
				{Key: match.MediaTitleKey, Type: match.Matches, Value: "/Example/"},
			}),
			isEligible: true,
		},
	}

	runWorkflowTests(t, container, tests)
}
