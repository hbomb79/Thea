package transcodes

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/api/gen"
	"github.com/hbomb79/Thea/internal/api/util"
	"github.com/hbomb79/Thea/internal/transcode"
	"github.com/labstack/echo/v4"
)

type (
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
		GetTranscodesForMedia(transcodeID uuid.UUID) ([]*transcode.Transcode, error)
		GetTranscode(transcodeID uuid.UUID) *transcode.Transcode
		GetAllTranscodes() ([]*transcode.Transcode, error)
		DeleteTranscode(transcodeID uuid.UUID) error
	}

	TranscodesController struct {
		transcodeService TranscodeService
		store            Store
	}
)

func New(transcodeService TranscodeService, store Store) *TranscodesController {
	return &TranscodesController{transcodeService: transcodeService, store: store}
}

func (controller *TranscodesController) CreateTranscodeTask(ec echo.Context, request gen.CreateTranscodeTaskRequestObject) (gen.CreateTranscodeTaskResponseObject, error) {
	if err := controller.transcodeService.NewTask(request.Body.MediaId, request.Body.TargetId); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Task creation failed: %v", err))
	}

	return gen.CreateTranscodeTask201Response{}, nil
}

func (controller *TranscodesController) ListActiveTranscodeTasks(ec echo.Context, request gen.ListActiveTranscodeTasksRequestObject) (gen.ListActiveTranscodeTasksResponseObject, error) {
	tasks := controller.transcodeService.AllTasks()

	return gen.ListActiveTranscodeTasks200JSONResponse(util.ApplyConversion(tasks, NewDtoFromTask)), nil
}

func (controller *TranscodesController) ListCompletedTranscodeTasks(ec echo.Context, request gen.ListCompletedTranscodeTasksRequestObject) (gen.ListCompletedTranscodeTasksResponseObject, error) {
	tasks, err := controller.store.GetAllTranscodes()
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return gen.ListCompletedTranscodeTasks200JSONResponse(util.ApplyConversion(tasks, NewDtoFromModel)), nil
}

func (controller *TranscodesController) GetTranscodeTask(ec echo.Context, request gen.GetTranscodeTaskRequestObject) (gen.GetTranscodeTaskResponseObject, error) {
	if task := controller.transcodeService.Task(request.Id); task != nil {
		return gen.GetTranscodeTask200JSONResponse(NewDtoFromTask(task)), nil
	}

	if model := controller.store.GetTranscode(request.Id); model != nil {
		return gen.GetTranscodeTask200JSONResponse(NewDtoFromModel(model)), nil
	}

	return nil, echo.ErrNotFound
}

func (controller *TranscodesController) PauseTranscodeTask(ec echo.Context, request gen.PauseTranscodeTaskRequestObject) (gen.PauseTranscodeTaskResponseObject, error) {
	if err := controller.transcodeService.PauseTask(request.Id); err != nil {
		if errors.Is(err, transcode.ErrTaskNotFound) {
			return nil, echo.ErrNotFound
		} else {
			return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to pause task %s: %s", request.Id, err))
		}
	}

	return gen.PauseTranscodeTask200Response{}, nil
}

func (controller *TranscodesController) ResumeTranscodeTask(ec echo.Context, request gen.ResumeTranscodeTaskRequestObject) (gen.ResumeTranscodeTaskResponseObject, error) {
	if err := controller.transcodeService.ResumeTask(request.Id); err != nil {
		if errors.Is(err, transcode.ErrTaskNotFound) {
			return nil, echo.ErrNotFound
		} else {
			return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to resume task %s: %s", request.Id, err))
		}
	}

	return gen.ResumeTranscodeTask200Response{}, nil
}

func (controller *TranscodesController) DeleteTranscodeTask(ec echo.Context, request gen.DeleteTranscodeTaskRequestObject) (gen.DeleteTranscodeTaskResponseObject, error) {
	// Try cancel active task - if not found, try delete completed task - if both not found
	// then error 404, else return the first error we encounter.
	if err := controller.transcodeService.CancelTask(request.Id); err != nil {
		if errors.Is(err, transcode.ErrTaskNotFound) {
			if err := controller.store.DeleteTranscode(request.Id); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return nil, echo.ErrNotFound
				}

				return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to delete completed task %s due to error: %v", request.Id, err))
			}

			return gen.DeleteTranscodeTask204Response{}, nil
		}

		return nil, echo.NewHTTPError(http.StatusBadRequest, "Failed to cancel task %s due to error: %v", request.Id, err)
	}

	return gen.DeleteTranscodeTask204Response{}, nil
}

// func (controller *TranscodesController) postTroubleResolution(ec echo.Context) error {
// 	return echo.NewHTTPError(http.StatusNotImplemented, "not yet implemented")
// }

// func (controller *TranscodesController) stream(ec echo.Context) error {
// 	return echo.NewHTTPError(http.StatusNotImplemented, "not yet implemented")
// }
