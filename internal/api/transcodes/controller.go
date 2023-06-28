package transcodes

import (
	"net/http"

	"github.com/hbomb79/Thea/internal/transcode"
	"github.com/labstack/echo/v4"
)

type (
	Dto struct{}

	TranscodeStore interface {
		Foo() error
		Task() *transcode.TranscodeTask
	}

	Controller struct {
		Store TranscodeStore
	}
)

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
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) getActive(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) getComplete(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) get(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) cancel(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) postTroubleResolution(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) stream(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}
