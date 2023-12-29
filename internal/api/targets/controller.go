package targets

import (
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/labstack/echo/v4"
)

type (
	Dto struct {
		Id    uuid.UUID    `json:"id"`
		Label string       `json:"label"`
		Ext   string       `json:"extension"`
		Opts  *ffmpeg.Opts `json:"ffmpeg_options"`
	}

	CreateRequest struct {
		Label string      `json:"label" validate:"required,alphaNumericWhitespaceTrimmed"`
		Ext   string      `json:"extension" validate:"required,alphanum"`
		Opts  ffmpeg.Opts `json:"ffmpeg_options" validate:"required"`
	}

	UpdateRequest struct {
		Label *string      `json:"label" validate:"omitempty,alphaNumericWhitespaceTrimmed"`
		Ext   *string      `json:"extension" validate:"omitempty,alphanum"`
		Opts  *ffmpeg.Opts `json:"ffmpeg_options"`
	}

	Store interface {
		SaveTarget(*ffmpeg.Target) error
		GetTarget(uuid.UUID) *ffmpeg.Target
		GetAllTargets() []*ffmpeg.Target
		DeleteTarget(uuid.UUID)
	}

	Controller struct {
		store     Store
		validator *validator.Validate
	}
)

func New(validate *validator.Validate, store Store) *Controller {
	return &Controller{store: store, validator: validate}
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
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid body: %v", err))
	}

	if err := controller.validator.Struct(createRequest); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid body: %v", err))
	}

	newTarget := ffmpeg.Target{
		ID:            uuid.New(),
		Label:         createRequest.Label,
		FfmpegOptions: &createRequest.Opts,
		Ext:           createRequest.Ext,
	}

	if err := controller.store.SaveTarget(&newTarget); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to save target: %v", err))
	}

	return ec.NoContent(http.StatusCreated)
}

func (controller *Controller) list(ec echo.Context) error {
	targets := controller.store.GetAllTargets()
	dtos := make([]*Dto, len(targets))
	for i, t := range targets {
		dtos[i] = NewDto(t)
	}

	return ec.JSON(http.StatusOK, dtos)
}

func (controller *Controller) get(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Target ID is not a valid UUID")
	}

	item := controller.store.GetTarget(id)
	if item == nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}

	return ec.JSON(http.StatusOK, NewDto(item))
}

func (controller *Controller) update(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Target ID is not a valid UUID")
	}

	var patchRequest UpdateRequest
	if err := ec.Bind(&patchRequest); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid body: %v", err))
	}
	if err := controller.validator.Struct(patchRequest); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid body: %v", err))
	}

	model := *controller.store.GetTarget(id)
	if patchRequest.Ext != nil {
		model.Ext = *patchRequest.Ext
	}
	if patchRequest.Label != nil {
		model.Label = *patchRequest.Label
	}
	if patchRequest.Opts != nil {
		model.FfmpegOptions = patchRequest.Opts
	}

	if err := controller.store.SaveTarget(&model); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to save target: %v", err))
	}

	return ec.NoContent(http.StatusOK)
}

func (controller *Controller) delete(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Target ID is not a valid UUID")
	}

	controller.store.DeleteTarget(id)
	return ec.NoContent(http.StatusNoContent)
}

func NewDto(model *ffmpeg.Target) *Dto {
	return &Dto{Id: model.ID, Label: model.Label, Ext: model.Ext, Opts: model.FfmpegOptions}
}
