package targets

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/labstack/echo/v4"
)

const (
	alphaNumericWhitespaceRegexString = "^[a-zA-Z0-9\\s]+$"
)

var (
	alphaNumericWhitespaceRegex = regexp.MustCompile(alphaNumericWhitespaceRegexString)
)

type (
	Dto struct {
		Id    uuid.UUID    `json:"id"`
		Label string       `json:"label"`
		Ext   string       `json:"extension"`
		Opts  *ffmpeg.Opts `json:"ffmpeg_opts"`
	}

	CreateRequest struct {
		Label string      `json:"label" validate:"required,alphaNumWhitespaceTrimmed"`
		Ext   string      `json:"extension" validate:"required,alphanum"`
		Opts  ffmpeg.Opts `json:"ffmpeg_opts" validate:"required"`
	}

	UpdateRequest struct {
		Label *string      `json:"label" validate:"omitempty,alphaNumWhitespaceTrimmed"`
		Ext   *string      `json:"extension" validate:"omitempty,alphanum"`
		Opts  *ffmpeg.Opts `json:"ffmpeg_opts"`
	}

	Store interface {
		SaveTarget(*ffmpeg.Target) error
		GetTarget(uuid.UUID) *ffmpeg.Target
		GetAllTargets() []*ffmpeg.Target
		DeleteTarget(uuid.UUID)
	}

	Controller struct {
		Store     Store
		validator *validator.Validate
	}
)

func New(store Store) *Controller {
	validate := validator.New()
	validate.RegisterValidation("alphaNumWhitespaceTrimmed", func(fl validator.FieldLevel) bool {
		str := fl.Field().String()
		if len(strings.TrimSpace(str)) != len(str) {
			return false
		}

		return alphaNumericWhitespaceRegex.MatchString(str)
	}, true)

	return &Controller{Store: store, validator: validate}
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

	if err := controller.validator.Struct(createRequest); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid body: %s", err.Error()))
	}

	newTarget := ffmpeg.Target{
		ID:            uuid.New(),
		Label:         createRequest.Label,
		FfmpegOptions: &createRequest.Opts,
		Ext:           createRequest.Ext,
	}

	if err := controller.Store.SaveTarget(&newTarget); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to save target: %s", err.Error()))
	}

	return ec.NoContent(http.StatusCreated)
}

func (controller *Controller) list(ec echo.Context) error {
	targets := controller.Store.GetAllTargets()
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

	item := controller.Store.GetTarget(id)
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
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid body: %s", err.Error()))
	}
	if err := controller.validator.Struct(patchRequest); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid body: %s", err.Error()))
	}

	model := *controller.Store.GetTarget(id)
	if patchRequest.Ext != nil {
		model.Ext = *patchRequest.Ext
	}
	if patchRequest.Label != nil {
		model.Label = *patchRequest.Label
	}
	if patchRequest.Opts != nil {
		model.FfmpegOptions = patchRequest.Opts
	}

	if err := controller.Store.SaveTarget(&model); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to save target: %s", err.Error()))
	}

	return ec.NoContent(http.StatusOK)
}

func (controller *Controller) delete(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Target ID is not a valid UUID")
	}

	controller.Store.DeleteTarget(id)
	return ec.NoContent(http.StatusNoContent)
}

func NewDto(model *ffmpeg.Target) *Dto {
	return &Dto{Id: model.ID, Label: model.Label, Ext: model.Ext, Opts: model.FfmpegOptions}
}