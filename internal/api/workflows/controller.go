package workflows

import (
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/workflow"
	"github.com/hbomb79/Thea/internal/workflow/match"
	"github.com/labstack/echo/v4"
)

type (
	CreateRequest struct {
		Label     string        `json:"label" validate:"required,alphaNumericWhitespaceTrimmed"`
		Enabled   bool          `json:"enabled" validate:"required"`
		TargetIDs []uuid.UUID   `json:"target_ids" validate:"required,min=1"`
		Criteria  []CriteriaDto `json:"criteria" validate:"required"`
	}

	UpdateRequest struct {
		Label     *string       `json:"label" validate:"omitempty,alphaNumericWhitespaceTrimmed"`
		Enabled   *bool         `json:"enabled"`
		TargetIDs *[]uuid.UUID  `json:"target_ids"`
		Criteria  []CriteriaDto `json:"criteria"`
	}

	WorkflowDto struct {
		ID        uuid.UUID     `json:"id"`
		Label     string        `json:"label"`
		Enabled   bool          `json:"enabled"`
		Criteria  []CriteriaDto `json:"criteria"`
		TargetIDs []uuid.UUID   `json:"target_ids"`
	}

	CriteriaDto struct {
		Key         match.Key         `json:"key"`
		Type        match.Type        `json:"type"`
		Value       string            `json:"value"`
		CombineType match.CombineType `json:"combine_type"`
	}

	Store interface {
		DeleteWorkflow(uuid.UUID)
		GetWorkflow(uuid.UUID) *workflow.Workflow
		GetAllWorkflows() []*workflow.Workflow
		CreateWorkflow(uuid.UUID, string, []match.Criteria, []uuid.UUID, bool) (*workflow.Workflow, error)
		UpdateWorkflow(uuid.UUID, *string, *[]match.Criteria, *[]uuid.UUID, *bool) (*workflow.Workflow, error)
		GetManyTargets(...uuid.UUID) []*ffmpeg.Target
	}

	Controller struct {
		Store    Store
		validate *validator.Validate
	}
)

func New(validate *validator.Validate, store Store) *Controller {
	return &Controller{Store: store, validate: validate}
}

func (controller *Controller) SetRoutes(eg *echo.Group) {
	eg.POST("/", controller.create)
	eg.GET("/", controller.list)
	eg.GET("/:id/", controller.get)
	eg.PATCH("/:id/", controller.update)
	eg.DELETE("/:id/", controller.delete)
}

func (controller *Controller) create(ec echo.Context) error {
	var createRequest CreateRequest
	if err := ec.Bind(&createRequest); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid body: %s", err.Error()))
	}

	if err := controller.validate.Struct(createRequest); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid body: %s", err.Error()))
	}

	workflowID := uuid.New()
	criteria := make([]match.Criteria, len(createRequest.Criteria))
	for i, v := range createRequest.Criteria {
		criteria[i] = NewCriteriaModel(workflowID, &v)
	}

	if model, err := controller.Store.CreateWorkflow(workflowID, createRequest.Label, criteria, createRequest.TargetIDs, createRequest.Enabled); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to create new workflow: %s", err.Error()))
	} else {
		return ec.JSON(http.StatusCreated, NewWorkflowDto(model))
	}
}

func (controller *Controller) list(ec echo.Context) error {
	workflowModels := controller.Store.GetAllWorkflows()
	workflowDtos := make([]WorkflowDto, len(workflowModels))
	for i, v := range workflowModels {
		workflowDtos[i] = *NewWorkflowDto(v)
	}

	return ec.JSON(http.StatusOK, workflowDtos)
}

func (controller *Controller) get(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Workflow ID is not a valid UUID")
	}

	workflow := controller.Store.GetWorkflow(id)
	if workflow == nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Workflow with ID %s does not exist", id))
	}

	return ec.JSON(http.StatusOK, NewWorkflowDto(workflow))
}

func (controller *Controller) update(ec echo.Context) error {
	workflowID, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Workflow ID is not a valid UUID")
	}

	var updateRequest UpdateRequest
	if err := ec.Bind(&updateRequest); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid body: %s", err.Error()))
	}

	if err := controller.validate.Struct(updateRequest); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid body: %s", err.Error()))
	}

	var criteriaToUpdate *[]match.Criteria = nil
	if updateRequest.Criteria != nil {
		criteria := make([]match.Criteria, len(updateRequest.Criteria))
		for i, v := range updateRequest.Criteria {
			criteria[i] = NewCriteriaModel(workflowID, &v)
		}

		criteriaToUpdate = &criteria
	}

	if model, err := controller.Store.UpdateWorkflow(workflowID, updateRequest.Label, criteriaToUpdate, updateRequest.TargetIDs, updateRequest.Enabled); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to update workflow: %s", err.Error()))
	} else {
		return ec.JSON(http.StatusOK, NewWorkflowDto(model))
	}
}

func (controller *Controller) delete(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Workflow ID is not a valid UUID")
	}

	controller.Store.DeleteWorkflow(id)
	return ec.NoContent(http.StatusNoContent)
}

func NewCriteriaModel(workflowID uuid.UUID, dto *CriteriaDto) match.Criteria {
	return match.Criteria{
		ID:          uuid.New(),
		Key:         dto.Key,
		Type:        dto.Type,
		Value:       dto.Value,
		CombineType: dto.CombineType,
	}
}

func NewCriteriaDto(model match.Criteria) CriteriaDto {
	return CriteriaDto{Key: model.Key, Type: model.Type, Value: model.Value, CombineType: model.CombineType}
}

func NewWorkflowDto(model *workflow.Workflow) *WorkflowDto {
	targetIDs := make([]uuid.UUID, len(model.Targets))
	for i, v := range model.Targets {
		targetIDs[i] = v.ID
	}

	criteriaDtos := make([]CriteriaDto, len(model.Criteria))
	for i, v := range model.Criteria {
		criteriaDtos[i] = NewCriteriaDto(v)
	}

	return &WorkflowDto{
		ID:        model.ID,
		Label:     model.Label,
		Enabled:   true,
		Criteria:  criteriaDtos,
		TargetIDs: targetIDs,
	}
}
