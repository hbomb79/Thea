package targets

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type (
	Dto struct{}

	Store interface{}

	Controller struct{ Store Store }
)

func New(store Store) *Controller {
	return &Controller{Store: store}
}

func (controller *Controller) SetRoutes(eg *echo.Group) {
	eg.POST("/", controller.create)
	eg.GET("/", controller.list)
	eg.GET("/:id/", controller.get)
	eg.PATCH("/:id/", controller.update)
	eg.DELETE("/:id/", controller.delete)
}

func (controller *Controller) create(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) list(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) get(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) update(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) delete(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}
