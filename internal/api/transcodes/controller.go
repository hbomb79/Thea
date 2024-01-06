package transcodes

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/transcode"
	"github.com/labstack/echo/v4"
)

type (
	createRequest struct {
		MediaID  *uuid.UUID `json:"media_id"`
		TargetID *uuid.UUID `json:"target_id"`
	}

	TranscodeDto struct {
		ID           uuid.UUID                     `json:"id"`
		MediaID      uuid.UUID                     `json:"media_id"`
		TargetId     uuid.UUID                     `json:"target_id"`
		OutputPath   string                        `json:"output_path"`
		Status       transcode.TranscodeTaskStatus `json:"status"`
		LastProgress *ffmpeg.Progress              `json:"last_progress,omitempty"`
	}

	TranscodeService interface {
		NewTask(mediaID uuid.UUID, targetID uuid.UUID) error
		CancelTask(id uuid.UUID) error
		PauseTask(id uuid.UUID) error
		ResumeTask(id uuid.UUID) error
		Task(id uuid.UUID) *transcode.TranscodeTask
		AllTasks() []*transcode.TranscodeTask
		ActiveTasksForMedia(mediaID uuid.UUID) []*transcode.TranscodeTask
	}

	Store interface {
		GetTranscodesForMedia(uuid.UUID) ([]*transcode.Transcode, error)
		GetTranscode(uuid.UUID) *transcode.Transcode
		GetAllTranscodes() ([]*transcode.Transcode, error)
		DeleteTranscode(id uuid.UUID) error
	}

	Controller struct {
		transcodeService TranscodeService
		store            Store
	}
)

func New(validate *validator.Validate, transcodeService TranscodeService, store Store) *Controller {
	return &Controller{transcodeService: transcodeService, store: store}
}

func (controller *Controller) SetRoutes(eg *echo.Group) {
	eg.POST("/", controller.create)
	eg.GET("/complete/", controller.getComplete)
	eg.GET("/active/", controller.getActive)
	eg.GET("/:id/", controller.get)
	eg.POST("/:id/pause/", controller.pause)
	eg.POST("/:id/resume/", controller.resume)
	eg.DELETE("/:id/", controller.cancel)
	eg.POST("/:id/trouble-resolution/", controller.postTroubleResolution)
	eg.GET("/:id/stream/", controller.stream)
}

func (controller *Controller) create(ec echo.Context) error {
	var createRequest createRequest
	if err := ec.Bind(&createRequest); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid body: %v", err))
	}

	mID := createRequest.MediaID
	tID := createRequest.TargetID
	if mID == nil || tID == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid body: one or both of mandatory fields 'media_id' and 'target_id' not provided")
	}

	if err := controller.transcodeService.NewTask(*mID, *tID); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Task creation failed: %v", err))
	}

	return ec.NoContent(http.StatusCreated)
}

func (controller *Controller) getActive(ec echo.Context) error {
	tasks := controller.transcodeService.AllTasks()
	taskDtos := make([]*TranscodeDto, len(tasks))
	for i, v := range tasks {
		taskDtos[i] = NewDtoFromTask(v)
	}

	return ec.JSON(http.StatusOK, taskDtos)
}

func (controller *Controller) getComplete(ec echo.Context) error {
	tasks, err := controller.store.GetAllTranscodes()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	taskDtos := make([]*TranscodeDto, len(tasks))
	for i, v := range tasks {
		taskDtos[i] = NewDtoFromModel(v)
	}

	return ec.JSON(http.StatusOK, taskDtos)
}

func (controller *Controller) get(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Task ID is not a valid UUID")
	}

	if task := controller.transcodeService.Task(id); task != nil {
		return ec.JSON(http.StatusOK, NewDtoFromTask(task))
	}

	if model := controller.store.GetTranscode(id); model != nil {
		return ec.JSON(http.StatusOK, NewDtoFromModel(model))
	}

	return echo.NewHTTPError(http.StatusNotFound)
}

func (controller *Controller) pause(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Task ID is not a valid UUID")
	}

	if err := controller.transcodeService.PauseTask(id); err != nil {
		if errors.Is(err, transcode.ErrTaskNotFound) {
			return echo.NewHTTPError(http.StatusNotFound)
		} else {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to pause task %s: %s", id, err))
		}
	}

	return ec.NoContent(http.StatusOK)
}

func (controller *Controller) resume(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Task ID is not a valid UUID")
	}

	if err := controller.transcodeService.ResumeTask(id); err != nil {
		if errors.Is(err, transcode.ErrTaskNotFound) {
			return echo.NewHTTPError(http.StatusNotFound)
		} else {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to resume task %s: %s", id, err))
		}
	}

	return ec.NoContent(http.StatusOK)
}

func (controller *Controller) cancel(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Task ID is not a valid UUID")
	}

	// Try cancel active task - if not found, try delete completed task - if both not found
	// then error 404, else return the first error we encounter.
	if err := controller.transcodeService.CancelTask(id); err != nil {
		if errors.Is(err, transcode.ErrTaskNotFound) {
			if err := controller.store.DeleteTranscode(id); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return echo.NewHTTPError(http.StatusNotFound)
				}

				return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to delete completed task %s due to error: %v", id, err))
			}

			return ec.NoContent(http.StatusOK)
		}

		return echo.NewHTTPError(http.StatusBadRequest, "Failed to cancel task %s due to error: %v", id, err)
	}

	return ec.NoContent(http.StatusOK)
}

func (controller *Controller) postTroubleResolution(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not yet implemented")
}

func (controller *Controller) stream(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not yet implemented")
}

func NewDtoFromModel(model *transcode.Transcode) *TranscodeDto {
	return &TranscodeDto{
		ID:           model.Id,
		MediaID:      model.MediaID,
		TargetId:     model.TargetID,
		OutputPath:   model.MediaPath,
		Status:       transcode.COMPLETE,
		LastProgress: nil,
	}
}

func NewDtoFromTask(model *transcode.TranscodeTask) *TranscodeDto {
	return &TranscodeDto{
		ID:           model.Id(),
		MediaID:      model.Media().Id(),
		TargetId:     model.Target().ID,
		OutputPath:   model.OutputPath(),
		Status:       model.Status(),
		LastProgress: model.LastProgress(),
	}
}
