package helpers

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/tests/gen"
	"github.com/labstack/gommon/random"
	"github.com/stretchr/testify/assert"
)

type APIClient struct {
	gen.ClientWithResponsesInterface
}

type Targets []gen.Target

func (ts Targets) IDs() []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(ts))
	for _, t := range ts {
		ids = append(ids, t.Id)
	}

	return ids
}

func (client *APIClient) CreateTarget(t *testing.T, label, extension string, ffmpegOpts map[string]any) gen.Target {
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

func (client *APIClient) CreateRandomTargets(t *testing.T, num int) Targets {
	targets := make([]gen.Target, 0, num)
	for range num {
		target := client.CreateTarget(t, random.String(24, random.Alphanumeric), "mp4", map[string]any{})
		targets = append(targets, target)

		t.Cleanup(func() { client.DeleteTarget(t, target.Id) })
	}

	return targets
}

func (client *APIClient) UpdateTarget(t *testing.T, targetID uuid.UUID, label, extension string, ffmpegOpts map[string]any) gen.Target {
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

func (client *APIClient) ListTargets(t *testing.T) []gen.Target {
	resp, err := client.ListTargetsWithResponse(ctx)
	assert.NoError(t, err, "failed to list targets: %v", err)
	assert.NotNil(t, resp, "failed to list targets: HTTP response was nil")
	assert.Equal(t, http.StatusOK, resp.StatusCode(), "failed to list targets: HTTP response status code was not as expected")
	assert.NotNil(t, resp.JSON200, "failed to list targets: JSON200 body nil")

	return *resp.JSON200
}

func (client *APIClient) GetTarget(t *testing.T, targetID uuid.UUID) gen.Target {
	resp, err := client.GetTargetWithResponse(ctx, targetID)
	assert.NoError(t, err, "failed to get target %s: %v", targetID, err)
	assert.NotNil(t, resp, "failed to get target %s: HTTP response was nil", targetID)
	assert.Equal(t, http.StatusOK, resp.StatusCode(), "failed to get target %s: HTTP response status code was not as expected", targetID)
	assert.NotNil(t, resp.JSON200, "failed to get target %s: JSON200 body nil", targetID)

	return *resp.JSON200
}

func (client *APIClient) DeleteTarget(t *testing.T, targetID uuid.UUID) {
	resp, err := client.DeleteTargetWithResponse(ctx, targetID)
	assert.NoError(t, err, "failed to delete target %s: %v", targetID, err)
	assert.NotNil(t, resp, "failed to delete target %s: HTTP response was nil", targetID)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode(), "failed to delete target %s: HTTP response status code was not as expected")
}

func (client *APIClient) CreateWorkflow(t *testing.T, criteria *[]gen.WorkflowCriteria, enabled bool, label string, targetIDs *[]uuid.UUID) gen.Workflow {
	resp, err := client.CreateWorkflowWithResponse(ctx, gen.CreateWorkflowRequest{Criteria: criteria, Enabled: enabled, Label: label, TargetIds: targetIDs})

	assert.NoError(t, err, "failed to create workflow %s: %v", label, err)
	assert.NotNil(t, resp, "failed to create workflow %s: HTTP response was nil", label)
	assert.Equal(t, http.StatusCreated, resp.StatusCode(), "failed to create workflow %s: HTTP response status code was not as expected", label)
	assert.NotNil(t, resp.JSON201, "failed to create workflow %s: JSON201 body nil", label)

	return *resp.JSON201
}

type (
	Boolean struct{ Bool bool }
	String  struct{ String string }
)

func (client *APIClient) UpdateWorkflow(t *testing.T, workflowID uuid.UUID,
	criteria *[]gen.WorkflowCriteria, enabled *Boolean, label string, targetIDs *[]uuid.UUID,
) gen.Workflow {
	updateDto := gen.UpdateWorkflowRequest{Criteria: criteria, TargetIds: targetIDs}
	if label != "" {
		updateDto.Label = &label
	}
	if enabled != nil {
		updateDto.Enabled = &enabled.Bool
	}

	resp, err := client.UpdateWorkflowWithResponse(ctx, workflowID, updateDto)

	assert.NoError(t, err, "failed to update workflow %s: %v", workflowID, err)
	assert.NotNil(t, resp, "failed to update workflow %s: HTTP response was nil", workflowID)
	assert.Equal(t, http.StatusOK, resp.StatusCode(), "failed to update workflow %s: HTTP response status code was not as expected", workflowID)
	assert.NotNil(t, resp.JSON200, "failed to update workflow %s: JSON200 body nil", workflowID)

	return *resp.JSON200
}

func (client *APIClient) ListWorkflows(t *testing.T) []gen.Workflow {
	resp, err := client.ListWorkflowsWithResponse(ctx)
	assert.NoError(t, err, "failed to list workflows: %v", err)
	assert.NotNil(t, resp, "failed to list workflows: HTTP response was nil")
	assert.Equal(t, http.StatusOK, resp.StatusCode(), "failed to list workflows: HTTP response status code was not as expected")
	assert.NotNil(t, resp.JSON200, "failed to list workflows: JSON200 body nil")

	return *resp.JSON200
}

func (client *APIClient) GetWorkflow(t *testing.T, workflowID uuid.UUID) gen.Workflow {
	resp, err := client.GetWorkflowWithResponse(ctx, workflowID)
	assert.NoError(t, err, "failed to get workflow %s: %v", workflowID, err)
	assert.NotNil(t, resp, "failed to get workflow %s: HTTP response was nil", workflowID)
	assert.Equal(t, http.StatusOK, resp.StatusCode(), "failed to get workflow %s: HTTP response status code was not as expected", workflowID)
	assert.NotNil(t, resp.JSON200, "failed to get workflow %s: JSON200 body nil", workflowID)

	return *resp.JSON200
}

func (client *APIClient) DeleteWorkflow(t *testing.T, workflowID uuid.UUID) {
	resp, err := client.DeleteWorkflowWithResponse(ctx, workflowID)
	assert.NoError(t, err, "failed to delete workflow %s: %v", workflowID, err)
	assert.NotNil(t, resp, "failed to delete workflow %s: HTTP response was nil", workflowID)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode(), "failed to delete workflow %s: HTTP response status code was not as expected")
}
