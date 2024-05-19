package integration_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/tests/gen"
	"github.com/hbomb79/Thea/tests/helpers"
	"github.com/labstack/gommon/random"
	"github.com/stretchr/testify/assert"
)

// TestWorkflow_CRUD performs some basic CRUD requests
// on the workflow resource.
func TestWorkflow_CRUD(t *testing.T) {
	srv := helpers.RequireThea(t, helpers.NewTheaServiceRequest())
	t.Parallel()

	_, client := srv.NewClientWithRandomUser(t)
	initialTargets := createRandomTargets(t, client, 3)
	workflow := createWorkflow(t, client, []gen.WorkflowCriteria{
		{CombineType: gen.OR, Key: gen.RESOLUTION, Type: gen.NOTEQUALS, Value: "10"},
	}, true, random.String(64), initialTargets.IDs())

	// Check creation DTO is correct compared to a subsequent fetch
	{
		list := listWorkflows(t, client)
		assert.Len(t, list, 1)
		assert.Equal(t, workflow, list[0], "Single entry in listed workflows does not equal created workflow")

		fetchedWorkflow := getWorkflow(t, client, workflow.Id)
		assert.Equal(t, workflow, fetchedWorkflow, "Fetched workflow does not equal created workflow")
	}

	// Partial update
	{
		updatedWorkflow := updateWorkflow(t, client, workflow.Id, nil, nil, "thiswasrenamedusingpartialupdating", nil)

		assert.NotEqual(t, workflow.Label, updatedWorkflow.Label, "Expected label of workflow to be updated")

		assert.Equal(t, workflow.Id, updatedWorkflow.Id, "ID of workflow changed after update")
		assert.Equal(t, workflow.Criteria, updatedWorkflow.Criteria, "Expected FfmpegOptions of workflow to not change during partial update of label")
		assert.Equal(t, workflow.TargetIds, updatedWorkflow.TargetIds, "Expected extension of workflow to not change during partial update of label")
		assert.Equal(t, workflow.Enabled, updatedWorkflow.Enabled, "Expected 'enabled' of workflow to not change during partial update of label")

		// Ensure response from UPDATE is the same as a subsequent GET
		assert.Equal(t, updatedWorkflow, getWorkflow(t, client, workflow.Id), "Updated workflow does not match that same workflow after fetching")
	}

	// Fully update workflow
	{
		newTargets := createRandomTargets(t, client, 3)
		targetIDs := newTargets.IDs()
		updatedWorkflow := updateWorkflow(t, client, workflow.Id, &[]gen.WorkflowCriteria{
			{CombineType: gen.AND, Key: gen.TITLE, Type: gen.EQUALS, Value: "atitle"},
		}, &optionalBool{false}, random.String(64), &targetIDs)

		assert.Equal(t, workflow.Id, updatedWorkflow.Id, "ID of workflow changed after update")
		assert.NotEqual(t, workflow.Label, updatedWorkflow.Label, "Expected label of workflow to be updated")
		assert.NotEqual(t, workflow.Criteria, updatedWorkflow.Criteria, "Expected FfmpegOptions of workflow to change during full update")
		assert.NotEqual(t, workflow.TargetIds, updatedWorkflow.TargetIds, "Expected extension of workflow to change during full update")
		assert.NotEqual(t, workflow.Enabled, updatedWorkflow.Enabled, "Expected 'enabled' of workflow to change during full update")

		// Ensure response from UPDATE is the same as a subsequent GET
		assert.Equal(t, updatedWorkflow, getWorkflow(t, client, workflow.Id), "Updated workflow does not match that same workflow after fetching")
	}

	// Delete workflow
	deleteWorkflow(t, client, workflow.Id)

	// Ensure it's no longer listed
	assert.Len(t, listWorkflows(t, client), 0)

	// ... And that fetching is a 404
	resp, err := client.GetWorkflowWithResponse(ctx, workflow.Id)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	assert.Nil(t, resp.JSON200)
}

func TestWorkflow_Creation(t *testing.T) {
	srv := helpers.RequireThea(t, helpers.NewTheaServiceRequest())
	_, client := srv.NewClientWithRandomUser(t)
	tests := []struct {
		Summary       string
		ShouldSucceed bool
		Label         string
		Enabled       bool
		Criteria      []gen.WorkflowCriteria
		TargetIDs     []uuid.UUID
	}{
		{
			Summary:       "",
			ShouldSucceed: false,
			Label:         "",
			Enabled:       false,
			Criteria:      []gen.WorkflowCriteria{},
			TargetIDs:     []uuid.UUID{},
		},
		{
			Summary:       "",
			ShouldSucceed: false,
			Label:         "",
			Enabled:       false,
			Criteria:      []gen.WorkflowCriteria{},
			TargetIDs:     []uuid.UUID{},
		},
		{
			Summary:       "",
			ShouldSucceed: false,
			Label:         "",
			Enabled:       false,
			Criteria:      []gen.WorkflowCriteria{},
			TargetIDs:     []uuid.UUID{},
		},
		{
			Summary:       "",
			ShouldSucceed: false,
			Label:         "",
			Enabled:       false,
			Criteria:      []gen.WorkflowCriteria{},
			TargetIDs:     []uuid.UUID{},
		},
		{
			Summary:       "",
			ShouldSucceed: false,
			Label:         "",
			Enabled:       false,
			Criteria:      []gen.WorkflowCriteria{},
			TargetIDs:     []uuid.UUID{},
		},
		{
			Summary:       "",
			ShouldSucceed: false,
			Label:         "",
			Enabled:       false,
			Criteria:      []gen.WorkflowCriteria{},
			TargetIDs:     []uuid.UUID{},
		},
		{
			Summary:       "",
			ShouldSucceed: false,
			Label:         "",
			Enabled:       false,
			Criteria:      []gen.WorkflowCriteria{},
			TargetIDs:     []uuid.UUID{},
		},
		{
			Summary:       "",
			ShouldSucceed: false,
			Label:         "",
			Enabled:       false,
			Criteria:      []gen.WorkflowCriteria{},
			TargetIDs:     []uuid.UUID{},
		},
		{
			Summary:       "",
			ShouldSucceed: false,
			Label:         "",
			Enabled:       false,
			Criteria:      []gen.WorkflowCriteria{},
			TargetIDs:     []uuid.UUID{},
		},
		{
			Summary:       "",
			ShouldSucceed: false,
			Label:         "",
			Enabled:       false,
			Criteria:      []gen.WorkflowCriteria{},
			TargetIDs:     []uuid.UUID{},
		},
	}

	for _, test := range tests {
		t.Run(test.Summary, func(t *testing.T) {
			t.Parallel()

			if test.ShouldSucceed {
				wkflw := createWorkflow(t, client, test.Criteria, test.Enabled, test.Label, test.TargetIDs)
				assert.Equalf(t, test.Label, wkflw.Label, "creation of workflow failed: expected 'Label' to be '%v' but found '%v'", test.Label, wkflw.Label)
				assert.Equalf(t, test.Enabled, wkflw.Enabled, "creation of workflow failed: expected 'Enabled' to be '%v' but found '%v'", test.Enabled, wkflw.Enabled)
				assert.Equalf(t, test.TargetIDs, wkflw.TargetIds, "creation of workflow failed: expected 'TargetIds' to be '%v' but found '%v'", test.TargetIDs, wkflw.TargetIds)
				assert.Equalf(t, test.Criteria, wkflw.Criteria, "creation of workflow failed: expected 'Criteria' to be '%v' but found '%v'", test.Criteria, wkflw.Criteria)
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

func TestWorkflow_Update(t *testing.T) {
	t.SkipNow()
	// TODO
}

func TestWorkflow_ManageTargets(t *testing.T) {
	t.SkipNow()
	// TODO
}

// TestWorkflow_Criteria tests that workflows with certain
// criteria set on them correctly automatically initiate transcoding tasks
// for media which matches that criteria.
func TestWorkflow_Criteria(t *testing.T) {
	t.SkipNow()
	// TODO
	// Enabled, Single criteria
	// Enabled, Combined criteria (AND)
	// Enabled, Combined criteria (OR)
	// Disabled workflow has no effect
}

func createWorkflow(t *testing.T, client gen.ClientWithResponsesInterface, criteria []gen.WorkflowCriteria, enabled bool, label string, targetIDs []uuid.UUID) gen.Workflow {
	resp, err := client.CreateWorkflowWithResponse(ctx, gen.CreateWorkflowRequest{Criteria: criteria, Enabled: enabled, Label: label, TargetIds: targetIDs})

	assert.NoError(t, err, "failed to create workflow %s: %v", label, err)
	assert.NotNil(t, resp, "failed to create workflow %s: HTTP response was nil", label)
	assert.Equal(t, http.StatusCreated, resp.StatusCode(), "failed to create workflow %s: HTTP response status code was not as expected", label)
	assert.NotNil(t, resp.JSON201, "failed to create workflow %s: JSON201 body nil", label)

	return *resp.JSON201
}

type optionalBool struct{ bool bool }

func updateWorkflow(t *testing.T, client gen.ClientWithResponsesInterface, workflowID uuid.UUID,
	criteria *[]gen.WorkflowCriteria, enabled *optionalBool, label string, targetIDs *[]uuid.UUID,
) gen.Workflow {
	updateDto := gen.UpdateWorkflowRequest{Criteria: criteria, TargetIds: targetIDs}
	if label != "" {
		updateDto.Label = &label
	}
	if enabled != nil {
		updateDto.Enabled = &enabled.bool
	}

	resp, err := client.UpdateWorkflowWithResponse(ctx, workflowID, updateDto)

	assert.NoError(t, err, "failed to update workflow %s: %v", workflowID, err)
	assert.NotNil(t, resp, "failed to update workflow %s: HTTP response was nil", workflowID)
	assert.Equal(t, http.StatusOK, resp.StatusCode(), "failed to update workflow %s: HTTP response status code was not as expected", workflowID)
	assert.NotNil(t, resp.JSON200, "failed to update workflow %s: JSON200 body nil", workflowID)

	return *resp.JSON200
}

func listWorkflows(t *testing.T, client gen.ClientWithResponsesInterface) []gen.Workflow {
	resp, err := client.ListWorkflowsWithResponse(ctx)
	assert.NoError(t, err, "failed to list workflows: %v", err)
	assert.NotNil(t, resp, "failed to list workflows: HTTP response was nil")
	assert.Equal(t, http.StatusOK, resp.StatusCode(), "failed to list workflows: HTTP response status code was not as expected")
	assert.NotNil(t, resp.JSON200, "failed to list workflows: JSON200 body nil")

	return *resp.JSON200
}

func getWorkflow(t *testing.T, client gen.ClientWithResponsesInterface, workflowID uuid.UUID) gen.Workflow {
	resp, err := client.GetWorkflowWithResponse(ctx, workflowID)
	assert.NoError(t, err, "failed to get workflow %s: %v", workflowID, err)
	assert.NotNil(t, resp, "failed to get workflow %s: HTTP response was nil", workflowID)
	assert.Equal(t, http.StatusOK, resp.StatusCode(), "failed to get workflow %s: HTTP response status code was not as expected", workflowID)
	assert.NotNil(t, resp.JSON200, "failed to get workflow %s: JSON200 body nil", workflowID)

	return *resp.JSON200
}

func deleteWorkflow(t *testing.T, client gen.ClientWithResponsesInterface, workflowID uuid.UUID) {
	resp, err := client.DeleteWorkflowWithResponse(ctx, workflowID)
	assert.NoError(t, err, "failed to delete workflow %s: %v", workflowID, err)
	assert.NotNil(t, resp, "failed to delete workflow %s: HTTP response was nil", workflowID)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode(), "failed to delete workflow %s: HTTP response status code was not as expected")
}
