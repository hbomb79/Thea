package lists

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type (
	Dto struct{}

	ListsStore interface{}

	Controller struct {
		Store ListsStore
	}
)

func (controller *Controller) SetupRoutes(eg echo.Group) {
	eg.POST("/", controller.create)
	eg.GET("/", controller.list)
	eg.GET("/:id/", controller.get)
	eg.PATCH("/:id/", controller.update)
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
