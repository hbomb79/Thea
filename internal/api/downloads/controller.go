package downloads

import "github.com/labstack/echo/v4"

type (
	Dto   struct{}
	Store interface {
	}
	Controller struct {
		Store Store
	}
)

func (controller *Controller) Init(eg *echo.Group) {
	eg.GET("/", controller.list)
	eg.GET("/:id/", controller.get)
	eg.DELETE("/:id/", controller.delete)
	eg.POST("/:id/approve", controller.postApproval)
	eg.POST("/:id/trouble-resolution", controller.postTroubleResolution)
}

func (controller *Controller) list(ctx echo.Context) error                  { return nil }
func (controller *Controller) get(ctx echo.Context) error                   { return nil }
func (controller *Controller) delete(ctx echo.Context) error                { return nil }
func (controller *Controller) postApproval(ctx echo.Context) error          { return nil }
func (controller *Controller) postTroubleResolution(ctx echo.Context) error { return nil }
