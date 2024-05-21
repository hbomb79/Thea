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
		updatedWorkflow := client.UpdateWorkflow(t, workflow.Id, nil, nil, "thiswasrenamedusingpartialupdating", nil)

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
			{CombineType: gen.AND, Key: gen.TITLE, Type: gen.EQUALS, Value: "atitle"},
		}, &helpers.Boolean{}, random.String(64), &targetIDs)

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
	srv := helpers.RequireThea(t, helpers.NewTheaServiceRequest())
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
				{CombineType: gen.AND, Key: gen.TITLE, Type: gen.NOTEQUALS, Value: "FooBar"},
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
				{CombineType: gen.AND, Key: gen.TITLE, Type: gen.EQUALS, Value: "FooBar"},
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
			Summary:       "Invalid criteria",
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
	t.SkipNow()
	srv := helpers.RequireThea(t, helpers.NewTheaServiceRequest())
	t.Parallel()

	_, client := srv.NewClientWithRandomUser(t)
	initialTargetIDs := client.CreateRandomTargets(t, 3).IDs()

	_ = client.CreateWorkflow(t, nil, true, "UpdateME", &initialTargetIDs)

	tests := []struct {
		Summary   string
		Label     *helpers.String
		Enabled   *helpers.Boolean
		Criteria  *[]gen.WorkflowCriteria
		TargetIDs *[]uuid.UUID
	}{
		{
			Summary:   "Valid update all fields",
			Label:     &helpers.String{String: "UpdatedME"},
			Enabled:   &helpers.Boolean{},
			Criteria:  &[]gen.WorkflowCriteria{},
			TargetIDs: &[]uuid.UUID{},
		},
		{
			Summary:   "Valid update label",
			Label:     &helpers.String{String: "UpdatedME"},
			Enabled:   &helpers.Boolean{},
			Criteria:  &[]gen.WorkflowCriteria{},
			TargetIDs: &[]uuid.UUID{},
		},
		{
			Summary:   "Valid update enabled",
			Label:     &helpers.String{String: "UpdatedME"},
			Enabled:   &helpers.Boolean{},
			Criteria:  &[]gen.WorkflowCriteria{},
			TargetIDs: &[]uuid.UUID{},
		},
		{
			Summary:   "Valid update criteria",
			Label:     &helpers.String{String: "UpdatedME"},
			Enabled:   &helpers.Boolean{},
			Criteria:  &[]gen.WorkflowCriteria{},
			TargetIDs: &[]uuid.UUID{},
		},
		{
			Summary:   "Valid update targets",
			Label:     &helpers.String{String: "UpdatedME"},
			Enabled:   &helpers.Boolean{},
			Criteria:  &[]gen.WorkflowCriteria{},
			TargetIDs: &[]uuid.UUID{},
		},
		{
			Summary:   "Invalid update label",
			Label:     &helpers.String{String: "UpdatedME"},
			Enabled:   &helpers.Boolean{},
			Criteria:  &[]gen.WorkflowCriteria{},
			TargetIDs: &[]uuid.UUID{},
		},
		{
			Summary:   "Invalid update criteria",
			Label:     &helpers.String{String: "UpdatedME"},
			Enabled:   &helpers.Boolean{},
			Criteria:  &[]gen.WorkflowCriteria{},
			TargetIDs: &[]uuid.UUID{},
		},
		{
			Summary:   "Invalid update targets",
			Label:     &helpers.String{},
			Enabled:   &helpers.Boolean{},
			Criteria:  &[]gen.WorkflowCriteria{},
			TargetIDs: &[]uuid.UUID{},
		},
	}

	for range tests {
	}
}

// TestWorkflow_Ingestion tests that workflows with certain
// criteria set on them correctly automatically initiate transcoding tasks
// for newly ingested media which matches that criteria.
func TestWorkflow_Ingestion(t *testing.T) {
	t.SkipNow()
	// TODO
	// Enabled, Single criteria
	// Enabled, Combined criteria (AND)
	// Enabled, Combined criteria (OR)
	// Disabled workflow has no effect
}
