package workflows

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/api/gen"
	"github.com/hbomb79/Thea/internal/api/util"
	"github.com/hbomb79/Thea/internal/workflow"
	"github.com/hbomb79/Thea/internal/workflow/match"
	"github.com/labstack/echo/v4"
)

type (
	Store interface {
		DeleteWorkflow(workflowID uuid.UUID)
		GetWorkflow(workflowID uuid.UUID) *workflow.Workflow
		GetAllWorkflows() []*workflow.Workflow
		CreateWorkflow(workflowID uuid.UUID, label string, criteria []match.Criteria, targetIDs []uuid.UUID, enabled bool) (*workflow.Workflow, error)
		UpdateWorkflow(workflowID uuid.UUID, newLabel *string, newCriteria *[]match.Criteria, newTargetIDs *[]uuid.UUID, newEnabled *bool) (*workflow.Workflow, error)
	}

	WorkflowController struct{ store Store }
)

func New(store Store) *WorkflowController {
	return &WorkflowController{store: store}
}

func (controller *WorkflowController) CreateWorkflow(ec echo.Context, request gen.CreateWorkflowRequestObject) (gen.CreateWorkflowResponseObject, error) {
	workflow, err := controller.store.CreateWorkflow(
		uuid.New(),
		request.Body.Label,
		util.ApplyConversion(util.NotNilOrDefault(request.Body.Criteria, []gen.WorkflowCriteria{}), criteriaToModel),
		util.NotNilOrDefault(request.Body.TargetIds, []uuid.UUID{}),
		request.Body.Enabled,
	)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to create new workflow: %v", err))
	}

	return gen.CreateWorkflow201JSONResponse(workflowToDto(workflow)), nil
}

func (controller *WorkflowController) ListWorkflows(ec echo.Context, request gen.ListWorkflowsRequestObject) (gen.ListWorkflowsResponseObject, error) {
	workflowModels := controller.store.GetAllWorkflows()

	return gen.ListWorkflows200JSONResponse(util.ApplyConversion(workflowModels, workflowToDto)), nil
}

func (controller *WorkflowController) GetWorkflow(ec echo.Context, request gen.GetWorkflowRequestObject) (gen.GetWorkflowResponseObject, error) {
	workflow := controller.store.GetWorkflow(request.Id)
	if workflow == nil {
		return nil, echo.ErrNotFound
	}

	return gen.GetWorkflow200JSONResponse(workflowToDto(workflow)), nil
}

func (controller *WorkflowController) UpdateWorkflow(ec echo.Context, request gen.UpdateWorkflowRequestObject) (gen.UpdateWorkflowResponseObject, error) {
	model, err := controller.store.UpdateWorkflow(
		request.Id,
		request.Body.Label,
		util.ApplyOptionalConversion(request.Body.Criteria, criteriaToModel),
		request.Body.TargetIds,
		request.Body.Enabled,
	)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to update workflow: %v", err))
	}

	return gen.UpdateWorkflow200JSONResponse(workflowToDto(model)), nil
}

func (controller *WorkflowController) DeleteWorkflow(ec echo.Context, request gen.DeleteWorkflowRequestObject) (gen.DeleteWorkflowResponseObject, error) {
	controller.store.DeleteWorkflow(request.Id)

	return gen.DeleteWorkflow204Response{}, nil
}
