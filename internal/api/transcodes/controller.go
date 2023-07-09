package transcodes

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/transcode"
	"github.com/labstack/echo/v4"
)

type (
	Dto struct{}

	Service interface {
		AllTasks() []*transcode.TranscodeTask
	}

	Store interface {
		GetTranscodesForMedia(uuid.UUID) ([]*transcode.TranscodeTask, error)
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
	return echo.NewHTTPError(http.StatusNotImplemented, "not yet implemented")
}

func (controller *Controller) getActive(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not yet implemented")
}

func (controller *Controller) getComplete(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not yet implemented")
}

func (controller *Controller) get(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not yet implemented")
}

func (controller *Controller) cancel(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not yet implemented")
}

func (controller *Controller) postTroubleResolution(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not yet implemented")
}

func (controller *Controller) stream(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not yet implemented")
}
