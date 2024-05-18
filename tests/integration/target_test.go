package integration_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/tests/gen"
	"github.com/hbomb79/Thea/tests/helpers"
	"github.com/stretchr/testify/assert"
)

// TestTarget_Complete tests the basic CRUD actions
// for the target resource all in one run.
func TestTarget_Complete(t *testing.T) {
	t.Parallel()

	srv := helpers.RequireThea(t, helpers.NewTheaServiceRequest())
	_, client := srv.NewClientWithRandomUser(t)

	// Create a target
	target := createTarget(t, client, "CRUD Target", "mp4", map[string]any{"Threads": 5})

	// Check creation DTO is correct compared to a subsequent fetch
	{
		list := listTargets(t, client)
		assert.Len(t, list, 1)
		assert.Equal(t, target, list[0], "Single entry in listed targets does not equal created target")

		fetchedTarget := getTarget(t, client, target.Id)
		assert.Equal(t, target, fetchedTarget, "Fetched target does not equal created target")
	}

	// Partial update
	{
		updatedTarget := updateTarget(t, client, target.Id, "thiswasrenamedusingpartialupdating", "", nil)

		assert.Equal(t, target.Id, updatedTarget.Id, "ID of target changed after update")
		assert.NotEqual(t, target.Label, updatedTarget.Label, "Expected label of target to be updated")
		assert.Equal(t, target.FfmpegOptions, updatedTarget.FfmpegOptions, "Expected FfmpegOptions of target to not change during partial update of label")
		assert.Equal(t, target.Extension, updatedTarget.Extension, "Expected extension of target to not change during partial update of label")

		// Ensure response from UPDATE is the same as a subsequent GET
		assert.Equal(t, updatedTarget, getTarget(t, client, target.Id), "Updated target does not match that same target after fetching")
	}

	// Fully update target
	{
		updatedTarget := updateTarget(t, client, target.Id, "thistargethasbeenrenamed", "mp5", map[string]any{
			"threads": 1,
		})

		assert.Equal(t, target.Id, updatedTarget.Id, "ID of target changed after update")
		assert.NotEqual(t, target.FfmpegOptions, updatedTarget.FfmpegOptions)
		assert.NotEqual(t, target.Label, updatedTarget.Label)
		assert.NotEqual(t, target.Extension, updatedTarget.Extension)

		// Ensure response from UPDATE is the same as a subsequent GET
		assert.Equal(t, updatedTarget, getTarget(t, client, target.Id), "Updated target does not match that same target after fetching")
	}

	// Delete target
	deleteTarget(t, client, target.Id)

	// Ensure it's no longer listed
	assert.Len(t, listTargets(t, client), 0)

	// ... And that fetching is a 404
	resp, err := client.GetTargetWithResponse(ctx, target.Id)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	assert.Nil(t, resp.JSON200)
}

func TestTarget_Creation(t *testing.T) {
	t.SkipNow()
	t.Parallel()
}

//nolint:gocognit
func TestTarget_Update(t *testing.T) {
	t.Parallel()

	srv := helpers.RequireThea(t, helpers.NewTheaServiceRequest())
	_, client := srv.NewClientWithRandomUser(t)

	// Create a target
	targetID := createTarget(t, client, "FooBar", "mp4", map[string]any{}).Id

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
				fetchedTarget := getTarget(t, client, targetID)
				assert.Equal(t, *resp.JSON200, fetchedTarget)
			} else {
				original := getTarget(t, client, targetID)

				resp, err := client.UpdateTargetWithResponse(ctx, targetID, updateDto)
				assert.NoError(t, err, "failed to update target %s: %v", targetID, err)
				assert.NotNil(t, resp, "failed to update target %s: HTTP response was nil", targetID)

				assert.Equal(t, http.StatusBadRequest, resp.StatusCode(), "failed to update target %s: HTTP response status code was not as expected", targetID)
				assert.Nil(t, resp.JSON200, "failed to update target %s: JSON200 body was non-nil when it was expected to be nil", targetID)

				// Ensure this 'failed' update didn't mess with anything
				assert.Equal(t, original, getTarget(t, client, targetID))
			}
		})
	}
}

func createTarget(t *testing.T, client gen.ClientWithResponsesInterface, label, extension string, ffmpegOpts map[string]any) gen.Target {
	resp, err := client.CreateTargetWithResponse(ctx, gen.CreateTargetRequest{
		Label:         label,
		Extension:     extension,
		FfmpegOptions: ffmpegOpts,
	})

	assert.NoError(t, err, "failed to create target %s: %v", label, err)
	assert.NotNil(t, resp, "failed to create target %s: HTTP response was nil", label)
	assert.Equal(t, http.StatusCreated, resp.StatusCode(), "failed to create target %s: HTTP response status code was not as expected", label)
	assert.NotNil(t, resp.JSON201, "failed to create target %s: JSON201 body nil", label)

	return *resp.JSON201
}

func updateTarget(t *testing.T, client gen.ClientWithResponsesInterface, targetID uuid.UUID, label, extension string, ffmpegOpts map[string]any) gen.Target {
	updateDto := gen.UpdateTargetRequest{}
	if label != "" {
		updateDto.Label = &label
	}
	if extension != "" {
		updateDto.Extension = &extension
	}
	if ffmpegOpts != nil {
		updateDto.FfmpegOptions = &ffmpegOpts
	}

	resp, err := client.UpdateTargetWithResponse(ctx, targetID, updateDto)

	assert.NoError(t, err, "failed to update target %s: %v", targetID, err)
	assert.NotNil(t, resp, "failed to update target %s: HTTP response was nil", targetID)
	assert.Equal(t, http.StatusOK, resp.StatusCode(), "failed to update target %s: HTTP response status code was not as expected", targetID)
	assert.NotNil(t, resp.JSON200, "failed to update target %s: JSON200 body nil", targetID)

	return *resp.JSON200
}

func listTargets(t *testing.T, client gen.ClientWithResponsesInterface) []gen.Target {
	resp, err := client.ListTargetsWithResponse(ctx)
	assert.NoError(t, err, "failed to list targets: %v", err)
	assert.NotNil(t, resp, "failed to list targets: HTTP response was nil")
	assert.Equal(t, http.StatusOK, resp.StatusCode(), "failed to list targets: HTTP response status code was not as expected")
	assert.NotNil(t, resp.JSON200, "failed to list targets: JSON200 body nil")

	return *resp.JSON200
}

func getTarget(t *testing.T, client gen.ClientWithResponsesInterface, targetID uuid.UUID) gen.Target {
	resp, err := client.GetTargetWithResponse(ctx, targetID)
	assert.NoError(t, err, "failed to get target %s: %v", targetID, err)
	assert.NotNil(t, resp, "failed to get target %s: HTTP response was nil", targetID)
	assert.Equal(t, http.StatusOK, resp.StatusCode(), "failed to get target %s: HTTP response status code was not as expected", targetID)
	assert.NotNil(t, resp.JSON200, "failed to get target %s: JSON200 body nil", targetID)

	return *resp.JSON200
}

func deleteTarget(t *testing.T, client gen.ClientWithResponsesInterface, targetID uuid.UUID) {
	resp, err := client.DeleteTargetWithResponse(ctx, targetID)
	assert.NoError(t, err, "failed to delete target %s: %v", targetID, err)
	assert.NotNil(t, resp, "failed to delete target %s: HTTP response was nil", targetID)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode(), "failed to delete target %s: HTTP response status code was not as expected")
}
