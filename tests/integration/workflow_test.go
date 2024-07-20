package integration_test

import (
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/http/websocket"
	"github.com/hbomb79/Thea/tests/gen"
	"github.com/hbomb79/Thea/tests/helpers"
	"github.com/hbomb79/go-chanassert"
	"github.com/labstack/gommon/random"
	"github.com/stretchr/testify/assert"
)

// TestWorkflow_CRUD performs some basic CRUD requests
// on the workflow resource.
func TestWorkflow_CRUD(t *testing.T) {
	srv := helpers.RequireThea(t, helpers.NewTheaServiceRequest())
	t.Parallel()

	_, client := srv.NewClientWithRandomUser(t)
	initialTargets := client.CreateRandomTargets(t, 3).IDs()
	workflow := client.CreateWorkflow(t, &[]gen.WorkflowCriteria{
		{CombineType: gen.OR, Key: gen.RESOLUTION, Type: gen.NOTEQUALS, Value: "10"},
	}, true, random.String(64), &initialTargets)

	// Check creation DTO is correct compared to a subsequent fetch
	{
		list := client.ListWorkflows(t)
		assert.Len(t, list, 1)
		assert.Equal(t, workflow, list[0], "Single entry in listed workflows does not equal created workflow")

		fetchedWorkflow := client.GetWorkflow(t, workflow.Id)
		assert.Equal(t, workflow, fetchedWorkflow, "Fetched workflow does not equal created workflow")
	}

	// Partial update
	{
		updatedWorkflow := client.UpdateWorkflow(t, workflow.Id, nil, nil, &helpers.String{String: "thiswasrenamedusingpartialupdating"}, nil)

		assert.NotEqual(t, workflow.Label, updatedWorkflow.Label, "Expected label of workflow to be updated")

		assert.Equal(t, workflow.Id, updatedWorkflow.Id, "ID of workflow changed after update")
		assert.Equal(t, workflow.Criteria, updatedWorkflow.Criteria, "Expected FfmpegOptions of workflow to not change during partial update of label")
		assert.Equal(t, workflow.TargetIds, updatedWorkflow.TargetIds, "Expected extension of workflow to not change during partial update of label")
		assert.Equal(t, workflow.Enabled, updatedWorkflow.Enabled, "Expected 'enabled' of workflow to not change during partial update of label")

		// Ensure response from UPDATE is the same as a subsequent GET
		assert.Equal(t, updatedWorkflow, client.GetWorkflow(t, workflow.Id), "Updated workflow does not match that same workflow after fetching")
	}

	{
		// Delete one of the targets currently associated with this workflow
		// and ensure the target is dropped by the workflow without problem.
		client.DeleteTarget(t, initialTargets[0])

		wrkflw := client.GetWorkflow(t, workflow.Id)
		assert.Len(t, wrkflw.TargetIds, 2, "expected workflow targets to be one less following deletion of associated target")
		assert.ElementsMatchf(t, initialTargets[1:], wrkflw.TargetIds, "expected workflow targets to be missing deleted target")
	}

	// Fully update workflow
	{
		newTargets := client.CreateRandomTargets(t, 3)
		targetIDs := newTargets.IDs()
		updatedWorkflow := client.UpdateWorkflow(t, workflow.Id, &[]gen.WorkflowCriteria{
			{CombineType: gen.AND, Key: gen.MEDIATITLE, Type: gen.EQUALS, Value: "atitle"},
		}, &helpers.Boolean{}, &helpers.String{String: random.String(64)}, &targetIDs)

		assert.Equal(t, workflow.Id, updatedWorkflow.Id, "ID of workflow changed after update")
		assert.NotEqual(t, workflow.Label, updatedWorkflow.Label, "Expected label of workflow to be updated")
		assert.NotEqual(t, workflow.Criteria, updatedWorkflow.Criteria, "Expected FfmpegOptions of workflow to change during full update")
		assert.NotEqual(t, workflow.TargetIds, updatedWorkflow.TargetIds, "Expected extension of workflow to change during full update")
		assert.NotEqual(t, workflow.Enabled, updatedWorkflow.Enabled, "Expected 'enabled' of workflow to change during full update")

		// Ensure response from UPDATE is the same as a subsequent GET
		assert.Equal(t, updatedWorkflow, client.GetWorkflow(t, workflow.Id), "Updated workflow does not match that same workflow after fetching")
	}

	// Delete workflow
	client.DeleteWorkflow(t, workflow.Id)

	// Ensure it's no longer listed
	assert.Len(t, client.ListWorkflows(t), 0)

	// ... And that fetching is a 404
	resp, err := client.GetWorkflowWithResponse(ctx, workflow.Id)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	assert.Nil(t, resp.JSON200)
}

func TestWorkflow_Creation(t *testing.T) {
	srv := helpers.RequireDefaultThea(t)
	t.Parallel()

	_, client := srv.NewClientWithRandomUser(t)

	targetIDs := client.CreateRandomTargets(t, 4).IDs()
	aIDs := targetIDs[:2]
	bIDs := targetIDs[2:]

	tests := []struct {
		Summary       string
		ShouldSucceed bool
		Label         string
		Enabled       bool
		Criteria      *[]gen.WorkflowCriteria
		TargetIDs     *[]uuid.UUID
	}{
		{
			Summary:       "Valid workflow with no targets or criteria",
			ShouldSucceed: true,
			Label:         "ValidMinimal",
			Enabled:       true,
		},
		{
			Summary:       "Valid workflow with all fields",
			ShouldSucceed: true,
			Label:         "ValidComplete",
			Enabled:       false,
			Criteria: &[]gen.WorkflowCriteria{
				{CombineType: gen.AND, Key: gen.MEDIATITLE, Type: gen.NOTEQUALS, Value: "FooBar"},
			},
			TargetIDs: &aIDs,
		},
		{
			Summary:       "Valid workflow with targets, no criteria",
			ShouldSucceed: true,
			Label:         "ValidNoCriteria",
			Enabled:       true,
			TargetIDs:     &bIDs,
		},
		{
			Summary:       "Valid workflow with criteria, no targets",
			ShouldSucceed: true,
			Label:         "ValidNoTargets",
			Enabled:       false,
			Criteria: &[]gen.WorkflowCriteria{
				{CombineType: gen.AND, Key: gen.MEDIATITLE, Type: gen.EQUALS, Value: "FooBar"},
			},
		},
		{
			Summary:       "Invalid targets",
			ShouldSucceed: false,
			Label:         "InvalidTarget",
			Enabled:       true,
			TargetIDs:     &[]uuid.UUID{uuid.New(), uuid.New()},
		},
		{
			Summary:       "Invalid criteria (schema violation)",
			ShouldSucceed: false,
			Label:         "InvalidCriteria",
			Enabled:       true,
			Criteria: &[]gen.WorkflowCriteria{
				{
					CombineType: "NOR",
					Key:         "SOME",
					Type:        "NOTATYPE",
					Value:       "helloworld",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Summary, func(t *testing.T) {
			t.Parallel()

			if test.ShouldSucceed {
				wkflw := client.CreateWorkflow(t, test.Criteria, test.Enabled, test.Label, test.TargetIDs)
				assert.Equalf(t, test.Label, wkflw.Label, "creation of workflow failed: expected 'Label' to be '%v' but found '%v'", test.Label, wkflw.Label)
				assert.Equalf(t, test.Enabled, wkflw.Enabled, "creation of workflow failed: expected 'Enabled' to be '%v' but found '%v'", test.Enabled, wkflw.Enabled)

				// When creating a workflow, the targets are optional in the request body, however
				// an empty array will be returned when fetching the workflow.
				if test.TargetIDs == nil {
					assert.Emptyf(t, wkflw.TargetIds, "creation of workflow failed: expected 'TargetIds' to be EMPTY (nil) but found '%v'", wkflw.TargetIds)
				} else {
					assert.ElementsMatchf(t, *test.TargetIDs, wkflw.TargetIds, "creation of workflow failed: expected 'TargetIds' to be '%v' but found '%v'", test.TargetIDs, wkflw.TargetIds)
				}

				// Same as targets above, criteria is an optional field in the create request,
				// but will be automatically set to an empty array and so we must account for that here.
				if test.Criteria == nil {
					assert.Emptyf(t, wkflw.Criteria, "creation of workflow failed: expected 'Criteria' to be EMPTY (nil) but found '%v'", wkflw.Criteria)
				} else {
					assert.ElementsMatchf(t, *test.Criteria, wkflw.Criteria, "creation of workflow failed: expected 'Criteria' to be '%v' but found '%v'", test.Criteria, wkflw.Criteria)
				}
			} else {
				resp, err := client.CreateWorkflowWithResponse(
					ctx,
					gen.CreateWorkflowRequest{Criteria: test.Criteria, Enabled: test.Enabled, Label: test.Label, TargetIds: test.TargetIDs},
				)
				assert.NoError(t, err, "creation of workflow unexectedly failed")
				assert.Nil(t, resp.JSON201, "creation of workflow unexpectedly succeeded: expected JSON201 body to be nil")
				assert.Equal(t, http.StatusBadRequest, resp.StatusCode(), "creation of workflow unexpectedly succeeded: status code incorrect")
			}
		})
	}
}

// TestWorkflow_Update tests the updating of existing workflows with
// arbitrary updates of varying correctness.
func TestWorkflow_Update(t *testing.T) {
	srv := helpers.RequireDefaultThea(t)
	t.Parallel()

	_, client := srv.NewClientWithRandomUser(t)
	initialTargetIDs := client.CreateRandomTargets(t, 3).IDs()

	workflow := client.CreateWorkflow(t, nil, true, random.String(64, random.Alphanumeric), &initialTargetIDs)

	tests := []struct {
		Summary       string
		Label         *helpers.String
		Enabled       *helpers.Boolean
		Criteria      *[]gen.WorkflowCriteria
		TargetIDs     *[]uuid.UUID
		ShouldSucceed bool
	}{
		{
			Summary: "Valid update all fields",
			Label:   &helpers.String{String: "UpdatedME"},
			Enabled: &helpers.Boolean{Bool: false},
			Criteria: &[]gen.WorkflowCriteria{
				{CombineType: gen.AND, Key: gen.MEDIATITLE, Type: gen.MATCHES, Value: "foobar"},
				{CombineType: gen.AND, Key: gen.RESOLUTION, Type: gen.MATCHES, Value: "1920x1080"},
				{CombineType: gen.AND, Key: gen.SOURCEEXTENSION, Type: gen.MATCHES, Value: ".mp4"},
			},
			TargetIDs:     &[]uuid.UUID{initialTargetIDs[0]},
			ShouldSucceed: true,
		},
		{
			Summary:       "Valid update label",
			Label:         &helpers.String{String: "This is valid too"},
			ShouldSucceed: true,
		},
		{
			Summary:       "Valid update enabled",
			Enabled:       &helpers.Boolean{Bool: false},
			ShouldSucceed: true,
		},
		{
			Summary: "Valid update criteria (order)",
			Criteria: &[]gen.WorkflowCriteria{
				{CombineType: gen.AND, Key: gen.SOURCEEXTENSION, Type: gen.MATCHES, Value: ".mp4"},
				{CombineType: gen.AND, Key: gen.MEDIATITLE, Type: gen.MATCHES, Value: "foobar"},
				{CombineType: gen.AND, Key: gen.RESOLUTION, Type: gen.MATCHES, Value: "1920x1080"},
			},
			ShouldSucceed: true,
		},
		{
			Summary:       "Valid update criteria",
			Criteria:      &[]gen.WorkflowCriteria{},
			ShouldSucceed: true,
		},
		{
			Summary:       "Valid update targets",
			TargetIDs:     &initialTargetIDs,
			ShouldSucceed: true,
		},
		{
			Summary: "Invalid update label",
			Label:   &helpers.String{String: " not valid "},
		},
		{
			Summary: "Invalid update criteria (schema violation)",
			Criteria: &[]gen.WorkflowCriteria{
				{CombineType: "NOTACOMBINETYPE", Key: gen.MEDIATITLE, Type: gen.EQUALS, Value: "foo"},
			},
		},
		{
			Summary:   "Invalid update targets (empty)",
			TargetIDs: &[]uuid.UUID{},
		},
		{
			Summary:   "Invalid update targets (not found)",
			TargetIDs: &[]uuid.UUID{uuid.New()},
		},
	}

	for _, test := range tests {
		t.Run(test.Summary, func(t *testing.T) {
			if test.ShouldSucceed {
				wkflw := client.UpdateWorkflow(t, workflow.Id, test.Criteria, test.Enabled, test.Label, test.TargetIDs)

				// Ensure each field we intended to update, did get updated
				if test.Label != nil {
					assert.Equalf(t, test.Label.String, wkflw.Label, "update of workflow failed: expected 'Label' to be '%v' but found '%v'", test.Label, wkflw.Label)
				}
				if test.Enabled != nil {
					assert.Equalf(t, test.Enabled.Bool, wkflw.Enabled, "update of workflow failed: expected 'Enabled' to be '%v' but found '%v'", test.Enabled, wkflw.Enabled)
				}
				if test.TargetIDs != nil {
					assert.ElementsMatchf(t, *test.TargetIDs, wkflw.TargetIds, "update of workflow failed: expected 'TargetIds' to be '%v' but found '%v'", test.TargetIDs, wkflw.TargetIds)
				}
				if test.Criteria != nil {
					assert.Equalf(t, *test.Criteria, wkflw.Criteria, "update of workflow failed: expected 'Criteria' to be '%v' but found '%v'", test.Criteria, wkflw.Criteria)
				}
			} else {
				resp, err := client.UpdateWorkflowWithResponse(
					ctx,
					workflow.Id,
					gen.UpdateWorkflowRequest{Criteria: test.Criteria, Enabled: test.Enabled.Value(), Label: test.Label.Value(), TargetIds: test.TargetIDs},
				)
				assert.NoError(t, err, "update of workflow unexectedly failed")
				assert.Nil(t, resp.JSON200, "update of workflow unexpectedly succeeded: expected JSON200 body to be nil")
				assert.Equal(t, http.StatusBadRequest, resp.StatusCode(), "update of workflow unexpectedly succeeded: status code incorrect")
			}
		})
	}
}

// TestWorkflow_Ingestion tests that workflows with certain
// criteria set on them correctly automatically initiate transcoding tasks
// for newly ingested media which matches that criteria.
//
//nolint:funlen,gocognit
func TestWorkflow_Ingestion(t *testing.T) {
	// All tests below share a ingestion directory, however
	// each test uses it's own Thea instance.
	ingestDir, paths := helpers.TempDirWithFiles(t, map[string]string{
		"./testdata/validmedia/short-sample.mkv": "Shaun.of.the.Dead.2004.mkv",
	})

	tests := []struct {
		summary                 string
		criteria                *[]gen.WorkflowCriteria
		enabled                 bool
		shouldInitiateTranscode bool
	}{
		{
			summary:                 "Enabled with no criteria",
			criteria:                nil,
			enabled:                 true,
			shouldInitiateTranscode: true,
		},
		{
			summary: "Enabled with matching simple criteria",
			criteria: &[]gen.WorkflowCriteria{
				{Key: gen.MEDIATITLE, Type: gen.MATCHES, Value: "Shaun of the Dead", CombineType: gen.AND},
			},
			enabled:                 true,
			shouldInitiateTranscode: true,
		},
		{
			summary: "Enabled with matching complex criteria",
			criteria: &[]gen.WorkflowCriteria{
				{Key: gen.MEDIATITLE, Type: gen.MATCHES, Value: "SIMPLE", CombineType: gen.OR},             // false OR
				{Key: gen.MEDIATITLE, Type: gen.MATCHES, Value: "Shaun of the Dead", CombineType: gen.AND}, // true AND
				{Key: gen.RESOLUTION, Type: gen.MATCHES, Value: "1920x1080", CombineType: gen.OR},          // false OR
				{Key: gen.MEDIATITLE, Type: gen.MATCHES, Value: "Shaun of the Dead", CombineType: gen.AND}, // true AND
				{Key: gen.RESOLUTION, Type: gen.MATCHES, Value: "1280x720", CombineType: gen.AND},          // true
			},
			enabled:                 true,
			shouldInitiateTranscode: true,
		},
		{
			summary: "Enabled with non-matching criteria",
			criteria: &[]gen.WorkflowCriteria{
				{Key: gen.MEDIATITLE, Type: gen.MATCHES, Value: "SIMPLE", CombineType: gen.OR},             // false OR
				{Key: gen.MEDIATITLE, Type: gen.MATCHES, Value: "Shaun of the Dead", CombineType: gen.AND}, // true AND
				{Key: gen.RESOLUTION, Type: gen.MATCHES, Value: "1920x1080", CombineType: gen.OR},          // false OR
				{Key: gen.MEDIATITLE, Type: gen.MATCHES, Value: "notthetitle", CombineType: gen.AND},       // false
			},
			enabled:                 true,
			shouldInitiateTranscode: false,
		},
		{
			summary:                 "Disabled with no criteria",
			criteria:                nil,
			enabled:                 false,
			shouldInitiateTranscode: false,
		},
	}

	for i, test := range tests {
		t.Run(test.summary, func(t *testing.T) {
			t.Parallel()

			req := helpers.NewTheaServiceRequest().
				WithIngestDirectory(ingestDir).
				RequiresTMDB().
				WithEnvironmentVariable("INGEST_MODTIME_THRESHOLD_SECONDS", "5").
				WithEnvironmentVariable("FORMAT_DEFAULT_OUTPUT_DIR", t.TempDir()).
				WithDatabaseName(fmt.Sprintf("workflow_ingestion_%d", i))

			srv := helpers.RequireThea(t, req)
			_, client := srv.NewClientWithRandomUser(t)

			// Activity stream should see:
			// - Welcome message (handled already)
			// - Ingestion complete message
			// - New media message
			// - Transcode update (IF the test expects transcodes to be created)
			combiners := []chanassert.Combiner[websocket.SocketMessage]{
				// We expect duplicates here; one for the ingestion creation, and another for it's completion.
				// However, due to debouncing of the events, by the time the first message is released, the ingestion
				// is likely to have completed already
				chanassert.AtLeastNOf(2, helpers.MatchMessageTitle("INGEST_UPDATE")),
				// We only expect a single event for the media creation
				chanassert.OneOf(helpers.MatchMovieEvent(paths[0])),
			}

			// Conditionally include transcode related combiners
			// for the activity expecter
			if test.shouldInitiateTranscode {
				combiners = append(
					combiners,
					chanassert.AtLeastNOfEach(2, helpers.MatchMessageTitle("TRANSCODE_TASK_UPDATE")))
			}

			exp := srv.ActivityExpecter(t).
				Ignore(helpers.MatchMessageTitle("TRANSCODE_TASK_PROGRESS_UPDATE")).
				Expect(combiners...)
			exp.Listen()

			// Create 3 targets, assign first 2 to the workflow
			const numTargetsToCreate = 3
			targets := client.CreateRandomTargets(t, numTargetsToCreate)
			targetIDs := targets.IDs()[:numTargetsToCreate-1]

			_ = client.CreateWorkflow(t, test.criteria, test.enabled, random.String(16, random.Alphanumeric), &targetIDs)

			// Ask ingest service to poll
			{
				resp, err := client.PollIngestsWithResponse(ctx)
				assert.NoError(t, err)
				assert.Equal(t, http.StatusOK, resp.StatusCode())
			}

			var mediaID uuid.UUID
			if !assert.EventuallyWithT(t, func(c *assert.CollectT) {
				// Get the media ID
				list := client.ListMedia(t)
				if !assert.Len(c, list, 1) {
					return
				}

				mediaID = list[0].Id
			}, 10*time.Second, 500*time.Millisecond, "Media list never became populated with expected item") {
				return
			}

			// Give Thea some time to kickoff the transcodes
			time.Sleep(1 * time.Second)

			// Check transcode service for matching transcodes
			transcodes := client.ListActiveTranscodeTasks(t)

			// Ensure each target has a transcode for our media
			if test.shouldInitiateTranscode {
				assert.Len(t, transcodes, len(targetIDs))
				targetsExpected := make([]uuid.UUID, 0, len(transcodes))
				for _, transcode := range transcodes {
					targetsExpected = append(targetsExpected, transcode.TargetId)
					assert.Equalf(t, mediaID, transcode.MediaId, "expected all transcodes to belong to the same media ID")
				}

				assert.ElementsMatchf(t, targetsExpected, targetIDs, "expected a transcode to be started for each of the specified target IDs")
			} else {
				assert.Empty(t, transcodes, "expected no transcodes to be initiated by this workflow")
			}

			// Poll the media endpoint for this transcode and ensure we see the correct watch target output
			resp, err := client.GetMovieWithResponse(ctx, mediaID)
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.NotNil(t, resp.JSON200)

			episode := resp.JSON200
			assert.Len(t, episode.WatchTargets, numTargetsToCreate+1) // +1 as we create a 'fake' watch target for 'direct streaming' of the content

			seenDirect := false
			for _, wt := range episode.WatchTargets {
				assert.True(t, wt.Enabled)

				//nolint:gocritic
				if wt.TargetId == nil {
					assert.False(t, seenDirect, "should only see one watch target with a nil target ID")
					seenDirect = true

					// In addition to each target, one other 'Direct' watch target will
					// be present which allows streaming of the source media file directly
					assert.Equal(t, "Direct", wt.DisplayName)
					assert.Nil(t, wt.TargetId)
					assert.True(t, wt.Ready)
				} else if test.shouldInitiateTranscode && slices.Contains(targetIDs, *wt.TargetId) {
					// Targets which were attached to a workflow which we expected
					// to initiate transcodes should be marked as not-ready pretranscodes
					assert.False(t, wt.Ready)
					assert.Equal(t, gen.PRETRANSCODE, wt.Type)
				} else if slices.Contains(targets.IDs(), *wt.TargetId) {
					// All other targets should be ready live transcodes
					assert.True(t, wt.Ready)
					assert.Equal(t, gen.LIVETRANSCODE, wt.Type)
				} else {
					t.Errorf("unexpected watch target '%+v'", wt)
				}
			}

			// Cancel the transcodes
			for _, transcode := range transcodes {
				t.Logf("Deleting (cancelling) transcode %v", transcode)
				resp, err := client.DeleteTranscodeTaskWithResponse(ctx, transcode.Id)
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, resp.StatusCode(), http.StatusNoContent)
			}

			exp.AssertSatisfied(t, time.Second*10)
		})
	}
}
