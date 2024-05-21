package integration_test

import (
	"net/http"
	"testing"

	"github.com/hbomb79/Thea/tests/gen"
	"github.com/hbomb79/Thea/tests/helpers"
	"github.com/stretchr/testify/assert"
)

// TestTarget_Complete tests the basic CRUD actions
// for the target resource all in one run.
func TestTarget_CRUD(t *testing.T) {
	srv := helpers.RequireThea(t, helpers.NewTheaServiceRequest())
	t.Parallel()

	// Create a target
	_, client := srv.NewClientWithRandomUser(t)
	target := client.CreateTarget(t, "CRUD Target", "mp4", map[string]any{"Threads": 5})

	// Check creation DTO is correct compared to a subsequent fetch
	{
		list := client.ListTargets(t)
		assert.Len(t, list, 1)
		assert.Equal(t, target, list[0], "Single entry in listed targets does not equal created target")

		fetchedTarget := client.GetTarget(t, target.Id)
		assert.Equal(t, target, fetchedTarget, "Fetched target does not equal created target")
	}

	// Partial update
	{
		updatedTarget := client.UpdateTarget(t, target.Id, "thiswasrenamedusingpartialupdating", "", nil)

		assert.Equal(t, target.Id, updatedTarget.Id, "ID of target changed after update")
		assert.NotEqual(t, target.Label, updatedTarget.Label, "Expected label of target to be updated")
		assert.Equal(t, target.FfmpegOptions, updatedTarget.FfmpegOptions, "Expected FfmpegOptions of target to not change during partial update of label")
		assert.Equal(t, target.Extension, updatedTarget.Extension, "Expected extension of target to not change during partial update of label")

		// Ensure response from UPDATE is the same as a subsequent GET
		assert.Equal(t, updatedTarget, client.GetTarget(t, target.Id), "Updated target does not match that same target after fetching")
	}

	// Fully update target
	{
		updatedTarget := client.UpdateTarget(t, target.Id, "thistargethasbeenrenamed", "mp5", map[string]any{
			"threads": 1,
		})

		assert.Equal(t, target.Id, updatedTarget.Id, "ID of target changed after update")
		assert.NotEqual(t, target.FfmpegOptions, updatedTarget.FfmpegOptions)
		assert.NotEqual(t, target.Label, updatedTarget.Label)
		assert.NotEqual(t, target.Extension, updatedTarget.Extension)

		// Ensure response from UPDATE is the same as a subsequent GET
		assert.Equal(t, updatedTarget, client.GetTarget(t, target.Id), "Updated target does not match that same target after fetching")
	}

	// Delete target
	client.DeleteTarget(t, target.Id)

	// Ensure it's no longer listed
	assert.Len(t, client.ListTargets(t), 0)

	// ... And that fetching is a 404
	resp, err := client.GetTargetWithResponse(ctx, target.Id)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	assert.Nil(t, resp.JSON200)
}

func TestTarget_Creation(t *testing.T) {
	srv := helpers.RequireThea(t, helpers.NewTheaServiceRequest())
	t.Parallel()

	_, client := srv.NewClientWithRandomUser(t)
	tests := []struct {
		Summary       string
		Args          gen.CreateTargetRequest
		ShouldSucceed bool
	}{
		{
			Summary:       "Valid creation of a target",
			ShouldSucceed: true,
			Args: gen.CreateTargetRequest{
				Extension: "mp4",
				Label:     "Hello World",
				FfmpegOptions: map[string]any{
					"Threads": 5,
				},
			},
		},
		{
			Summary:       "Invalid label (not whitespace trimmed)",
			ShouldSucceed: false,
			Args: gen.CreateTargetRequest{
				Extension:     "mp4",
				Label:         "  this aint trimmed  ",
				FfmpegOptions: map[string]any{},
			},
		},
		{
			Summary:       "Invalid label (non alphanumeric)",
			ShouldSucceed: false,
			Args: gen.CreateTargetRequest{
				Extension:     "mp4",
				Label:         "not&*#valid ",
				FfmpegOptions: map[string]any{},
			},
		},
		{
			Summary:       "Invalid extension",
			ShouldSucceed: false,
			Args: gen.CreateTargetRequest{
				Extension:     ".mp4",
				Label:         "Hello World",
				FfmpegOptions: map[string]any{},
			},
		},
		{
			Summary:       "Invalid ffmpeg options",
			ShouldSucceed: false,
			Args: gen.CreateTargetRequest{
				Extension: "mp4",
				Label:     "Hello World",
				FfmpegOptions: map[string]any{
					"Threads": "notanumberhuh",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Summary, func(t *testing.T) {
			if test.ShouldSucceed {
				resp, err := client.CreateTargetWithResponse(ctx, test.Args)
				assert.NoError(t, err, "failed to create target %s: %v", test.Args.Label, err)
				assert.NotNil(t, resp, "failed to create target %s: HTTP response was nil", test.Args.Label)
				assert.Equal(t, http.StatusCreated, resp.StatusCode(), "failed to create target %s: HTTP response status code was not as expected", test.Args.Label)
				assert.NotNil(t, resp.JSON201, "failed to create target %s: JSON201 body nil", test.Args.Label)

				// Check that the values match what we asked them to be
				assert.Equal(t, test.Args.Label, resp.JSON201.Label)
				assert.Equal(t, test.Args.Extension, resp.JSON201.Extension)
				for k, actual := range resp.JSON201.FfmpegOptions {
					if expected, ok := test.Args.FfmpegOptions[k]; ok {
						assert.EqualValuesf(t, expected, actual, "ffmpeg options key '%s' failed: expected value %v, but actual was %v", k, expected, actual)
					} else {
						assert.Nilf(t, actual, "ffmpeg options key '%s' failed: expected 'nil', but actual was %v", k, actual)
					}
				}

				// Ensure the respose we got from 'create' is the same as a subsequent fetch
				fetchedTarget := client.GetTarget(t, resp.JSON201.Id)
				assert.Equal(t, *resp.JSON201, fetchedTarget)
			} else {
				resp, err := client.CreateTargetWithResponse(ctx, test.Args)
				assert.NoError(t, err, "failed to create target %s: %v", test.Args.Label, err)
				assert.NotNil(t, resp, "failed to create target %s: HTTP response was nil", test.Args.Label)

				assert.Equal(t, http.StatusBadRequest, resp.StatusCode(), "failed to create target %s: HTTP response status code was not as expected", test.Args.Label)
				assert.Nil(t, resp.JSON201, "failed to create target %s: JSON201 body was non-nil when it was expected to be nil", test.Args.Label)
			}
		})
	}
}

//nolint:gocognit
func TestTarget_Update(t *testing.T) {
	srv := helpers.RequireThea(t, helpers.NewTheaServiceRequest())
	t.Parallel()

	// Create a target
	_, client := srv.NewClientWithRandomUser(t)
	targetID := client.CreateTarget(t, "FooBar", "mp4", map[string]any{}).Id

	// Try and run some updates by it
	tests := []struct {
		Summary       string
		Label         string
		Extension     string
		FfmpegOpts    map[string]any
		ShouldSucceed bool
	}{
		{
			Summary:       "Partial valid update to label",
			Label:         "BarFoo",
			ShouldSucceed: true,
		},
		{
			Summary:       "Partial invalid update to label",
			Label:         "not ? a va _ lid &*#47 name",
			ShouldSucceed: false,
		},
		{
			Summary:       "Partial valid update to extension",
			Extension:     "mp5",
			ShouldSucceed: true,
		},
		{
			Summary:       "Partial invalid update to extension",
			Extension:     ".mp4",
			ShouldSucceed: false,
		},
		{
			Summary: "Partial valid update to ffmpeg opts",
			FfmpegOpts: map[string]any{
				"Threads": 5,
			},
			ShouldSucceed: true,
		},
		{
			Summary: "Partial invalid update to ffmpeg opts",
			FfmpegOpts: map[string]any{
				"notaproperty": "shouldfail",
			},
			ShouldSucceed: false,
		},
	}

	for _, test := range tests {
		t.Run(test.Summary, func(t *testing.T) {
			updateDto := gen.UpdateTargetRequest{}
			if test.Label != "" {
				updateDto.Label = &test.Label
			}
			if test.Extension != "" {
				updateDto.Extension = &test.Extension
			}
			if test.FfmpegOpts != nil {
				updateDto.FfmpegOptions = &test.FfmpegOpts
			}

			if test.ShouldSucceed {
				resp, err := client.UpdateTargetWithResponse(ctx, targetID, updateDto)
				assert.NoError(t, err, "failed to update target %s: %v", targetID, err)
				assert.NotNil(t, resp, "failed to update target %s: HTTP response was nil", targetID)
				assert.Equal(t, http.StatusOK, resp.StatusCode(), "failed to update target %s: HTTP response status code was not as expected", targetID)
				assert.NotNil(t, resp.JSON200, "failed to update target %s: JSON200 body nil", targetID)

				// Check that the values match what we asked them to be
				if test.Label != "" {
					assert.Equal(t, test.Label, resp.JSON200.Label)
				}
				if test.Extension != "" {
					assert.Equal(t, test.Extension, resp.JSON200.Extension)
				}
				if test.FfmpegOpts != nil {
					// Check that the fields we specified were changed correctly. ALL other fields should be nil
					for k, actual := range resp.JSON200.FfmpegOptions {
						if expected, ok := test.FfmpegOpts[k]; ok {
							assert.EqualValuesf(t, expected, actual, "ffmpeg options key '%s' failed: expected value %v, but actual was %v", k, expected, actual)
						} else {
							assert.Nilf(t, actual, "ffmpeg options key '%s' failed: expected 'nil', but actual was %v", k, actual)
						}
					}
				}

				// Ensure the respose we got from 'update' is the same as a subsequent fetch
				fetchedTarget := client.GetTarget(t, targetID)
				assert.Equal(t, *resp.JSON200, fetchedTarget)
			} else {
				original := client.GetTarget(t, targetID)

				resp, err := client.UpdateTargetWithResponse(ctx, targetID, updateDto)
				assert.NoError(t, err, "failed to update target %s: %v", targetID, err)
				assert.NotNil(t, resp, "failed to update target %s: HTTP response was nil", targetID)

				assert.Equal(t, http.StatusBadRequest, resp.StatusCode(), "failed to update target %s: HTTP response status code was not as expected", targetID)
				assert.Nil(t, resp.JSON200, "failed to update target %s: JSON200 body was non-nil when it was expected to be nil", targetID)

				// Ensure this 'failed' update didn't mess with anything
				assert.Equal(t, original, client.GetTarget(t, targetID))
			}
		})
	}
}
