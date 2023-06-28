package medias

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type (
	Dto struct{}

	MediaStore interface{}

	Controller struct {
		Store MediaStore
	}
)

func (controller *Controller) SetupRoutes(eg echo.Group) {
	eg.GET("/latest/", controller.getLatest)

	eg.GET("/movie/:id/", controller.getMovie)
	eg.DELETE("/movie/:id/", controller.deleteMovie)

	eg.GET("/series/:id/", controller.getSeries)
	eg.DELETE("/series/:id/", controller.deleteSeries)

	eg.GET("/season/:id/", controller.getSeason)
	eg.DELETE("/season/:id/", controller.deleteSeason)

	eg.GET("/episode/:id/", controller.getEpisode)
	eg.DELETE("/episode/:id/", controller.deleteEpisode)
}

func (controller *Controller) getLatest(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) getEpisode(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) getMovie(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) getSeries(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) getSeason(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) deleteEpisode(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) deleteMovie(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) deleteSeries(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) deleteSeason(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}
