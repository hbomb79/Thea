package workflows

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/workflow"
	"github.com/labstack/echo/v4"
)

type (
	Dto struct {
		Label     string      `json:"label"`
		Enabled   bool        `json:"enabled"`
		TargetIDs []uuid.UUID `json:"target_ids"`
	}

	CreateRequest struct {
		Label     string      `json:"label" validate:"required,alphaNumericWhitespace"`
		Enabled   bool        `json:"enabled" validate:"required"`
		TargetIDs []uuid.UUID `json:"target_ids" validate:"required"`
	}

	UpdateRequest struct {
		Label     *string      `json:"label" validate:"omitempty,alphaNumericWhitespace"`
		Enabled   *bool        `json:"enabled"`
		TargetIDs *[]uuid.UUID `json:"target_ids"`
	}

	Store interface {
		GetWorkflow(uuid.UUID) *workflow.Workflow
		SaveWorkflow(*workflow.Workflow) error
		GetManyTargets(...uuid.UUID) []*ffmpeg.Target
	}

	Controller struct {
		Store Store
	}
)

func New(store Store) *Controller {
	return &Controller{Store: store}
}

func (controller *Controller) SetRoutes(eg *echo.Group) {
	eg.POST("/", controller.create)
	eg.GET("/", controller.list)
	eg.GET("/:id/", controller.get)
	eg.PATCH("/:id/", controller.update)
}

func (controller *Controller) create(ec echo.Context) error {
	// Fetch the workflow
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) list(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) get(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) update(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}
