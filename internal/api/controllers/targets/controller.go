package targets

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/api/gen"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/labstack/echo/v4"
	"github.com/mitchellh/mapstructure"
)

type (
	Store interface {
		SaveTarget(target *ffmpeg.Target) error
		GetTarget(targetID uuid.UUID) *ffmpeg.Target
		GetAllTargets() []*ffmpeg.Target
		DeleteTarget(targetID uuid.UUID)
	}

	TargetController struct {
		store Store
	}
)

func New(store Store) *TargetController {
	return &TargetController{store: store}
}

func (controller *TargetController) CreateTarget(ec echo.Context, request gen.CreateTargetRequestObject) (gen.CreateTargetResponseObject, error) {
	decoded, err := ffmpegOptsToModel(request.Body.FfmpegOptions)
	if err != nil {
		return nil, err
	}

	newTarget := ffmpeg.Target{ID: uuid.New(), Label: request.Body.Label, FfmpegOptions: decoded, Ext: request.Body.Extension}
	if err := controller.store.SaveTarget(&newTarget); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to create target: %v", err))
	}

	return gen.CreateTarget201JSONResponse(NewDto(&newTarget)), nil
}

func (controller *TargetController) ListTargets(ec echo.Context, request gen.ListTargetsRequestObject) (gen.ListTargetsResponseObject, error) {
	targets := controller.store.GetAllTargets()

	return gen.ListTargets200JSONResponse(NewDtos(targets)), nil
}

func (controller *TargetController) GetTarget(ec echo.Context, request gen.GetTargetRequestObject) (gen.GetTargetResponseObject, error) {
	target := controller.store.GetTarget(request.Id)
	if target == nil {
		return nil, echo.ErrNotFound
	}

	return gen.GetTarget200JSONResponse(NewDto(target)), nil
}

func (controller *TargetController) UpdateTarget(ec echo.Context, request gen.UpdateTargetRequestObject) (gen.UpdateTargetResponseObject, error) {
	model := *controller.store.GetTarget(request.Id)
	if request.Body.Extension != nil {
		model.Ext = *request.Body.Extension
	}
	if request.Body.Label != nil {
		model.Label = *request.Body.Label
	}
	if request.Body.FfmpegOptions != nil {
		if opts, err := ffmpegOptsToModel(*request.Body.FfmpegOptions); err == nil {
			model.FfmpegOptions = opts
		} else {
			return nil, err
		}
	}

	if err := controller.store.SaveTarget(&model); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to save target: %v", err))
	}

	return gen.UpdateTarget200JSONResponse(NewDto(&model)), nil
}

func (controller *TargetController) DeleteTarget(ec echo.Context, request gen.DeleteTargetRequestObject) (gen.DeleteTargetResponseObject, error) {
	controller.store.DeleteTarget(request.Id)

	return gen.DeleteTarget204Response{}, nil
}

func ffmpegOptsToModel(opts map[string]interface{}) (*ffmpeg.Opts, error) {
	var decoded ffmpeg.Opts
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{ErrorUnused: true, Result: &decoded})
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to create map decoder: %s", err))
	}

	if err := decoder.Decode(opts); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to save target: ffmpeg_options malformed: %s", err))
	}

	return &decoded, nil
}

func ffmpegOptsToDto(opts *ffmpeg.Opts) map[string]interface{} {
	var dto map[string]interface{}
	if err := mapstructure.Decode(opts, &dto); err != nil {
		panic("ffmpeg options cannot be decoded to map[string]interface{}")
	}

	return dto
}

func NewDto(model *ffmpeg.Target) gen.Target {
	return gen.Target{Id: model.ID, Label: model.Label, Extension: model.Ext, FfmpegOptions: ffmpegOptsToDto(model.FfmpegOptions)}
}

func NewDtos(models []*ffmpeg.Target) []gen.Target {
	dtos := make([]gen.Target, len(models))
	for k, v := range models {
		dtos[k] = gen.Target{Id: v.ID, Label: v.Label, Extension: v.Ext, FfmpegOptions: ffmpegOptsToDto(v.FfmpegOptions)}
	}

	return dtos
}
