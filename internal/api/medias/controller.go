package medias

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

type (
	Dto struct{}

	Store interface{}

	Controller struct {
		Store Store
	}
)

func New(validate *validator.Validate, store Store) *Controller {
	return &Controller{Store: store}
}

func (controller *Controller) SetRoutes(eg *echo.Group) {
	eg.GET("/latest/", controller.getLatest)

	eg.GET("/movie/", controller.listMovies)
	eg.GET("/movie/:id/", controller.getMovie)
	eg.DELETE("/movie/:id/", controller.deleteMovie)

	eg.GET("/series/", controller.listSeries)
	eg.GET("/series/:id/", controller.getSeries)
	eg.DELETE("/series/:id/", controller.deleteSeries)

	eg.GET("/season/", controller.listSeasons)
	eg.GET("/season/:id/", controller.getSeason)
	eg.DELETE("/season/:id/", controller.deleteSeason)

	eg.GET("/episode/", controller.listEpisodes)
	eg.GET("/episode/:id/", controller.getEpisode)
	eg.DELETE("/episode/:id/", controller.deleteEpisode)
}

func (controller *Controller) getLatest(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) listEpisodes(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) getEpisode(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) listMovies(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) getMovie(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) listSeries(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) getSeries(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) listSeasons(ec echo.Context) error {
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
