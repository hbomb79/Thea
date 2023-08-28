package transcodes

import (
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/transcode"
	"github.com/labstack/echo/v4"
)

type (
	CreateRequest struct {
		MediaID  uuid.UUID `json:"media_id"`
		TargetID uuid.UUID `json:"target_id"`
	}

	Dto struct {
		ID           uuid.UUID                     `json:"id"`
		MediaID      uuid.UUID                     `json:"media_id"`
		TargetId     uuid.UUID                     `json:"target_id"`
		OutputPath   string                        `json:"output_path"`
		Status       transcode.TranscodeTaskStatus `json:"status"`
		LastProgress *ffmpeg.Progress              `json:"last_progress"`
	}

	Service interface {
		NewTask(uuid.UUID, uuid.UUID) error
		CancelTask(uuid.UUID)
		Task(uuid.UUID) *transcode.TranscodeTask
		AllTasks() []*transcode.TranscodeTask
	}

	Store interface {
		GetTranscodesForMedia(uuid.UUID) ([]*transcode.TranscodeTask, error)
		GetTranscode(uuid.UUID) *transcode.TranscodeTask
		GetAllTranscodes() ([]*transcode.TranscodeTask, error)
	}

	Controller struct {
		Service Service
		Store   Store
	}
)

func New(validate *validator.Validate, service Service, store Store) *Controller {
	return &Controller{Service: service, Store: store}
}

func (controller *Controller) SetRoutes(eg *echo.Group) {
	eg.POST("/", controller.create)
	eg.GET("/complete/", controller.getComplete)
	eg.GET("/active/", controller.getActive)
	eg.GET("/:id/", controller.get)
	eg.DELETE("/:id/", controller.cancel)
	eg.POST("/:id/trouble-resolution/", controller.postTroubleResolution)
	eg.GET("/:id/stream/", controller.stream)
}

func (controller *Controller) create(ec echo.Context) error {
	var createRequest CreateRequest
	if err := ec.Bind(&createRequest); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid body: %v", err))
	}

	if err := controller.Service.NewTask(createRequest.MediaID, createRequest.TargetID); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Task creation failed: %v", err))
	}

	return ec.NoContent(http.StatusCreated)
}

func (controller *Controller) getActive(ec echo.Context) error {
	tasks := controller.Service.AllTasks()
	taskDtos := make([]Dto, len(tasks))
	for i, v := range tasks {
		taskDtos[i] = NewDto(v)
	}

	return ec.JSON(http.StatusOK, taskDtos)
}

func (controller *Controller) getComplete(ec echo.Context) error {
	tasks, err := controller.Store.GetAllTranscodes()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	taskDtos := make([]Dto, len(tasks))
	for i, v := range tasks {
		taskDtos[i] = NewDto(v)
	}

	return ec.JSON(http.StatusOK, taskDtos)
}

func (controller *Controller) get(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Task ID is not a valid UUID")
	}

	if task := controller.Service.Task(id); task != nil {
		return ec.JSON(http.StatusOK, NewDto(task))
	}

	if task := controller.Store.GetTranscode(id); task != nil {
		return ec.JSON(http.StatusOK, NewDto(task))
	}

	return echo.NewHTTPError(http.StatusNotFound)
}

func (controller *Controller) cancel(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Task ID is not a valid UUID")
	}

	controller.Service.CancelTask(id)
	return ec.NoContent(http.StatusOK)
}

func (controller *Controller) postTroubleResolution(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not yet implemented")
}

func (controller *Controller) stream(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not yet implemented")
}

func NewDto(model *transcode.TranscodeTask) Dto {
	return Dto{
		ID:           model.Id(),
		MediaID:      model.Media().Id(),
		TargetId:     model.Target().ID,
		OutputPath:   "NYI",
		Status:       model.Status(),
		LastProgress: model.LastProgress(),
	}
}
