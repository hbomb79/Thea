package transcodes

import "github.com/labstack/echo/v4"

type (
	TranscodeDto struct{}

	TargetDto struct{}

	FfmpegStore interface{}

	Controller struct {
		Store FfmpegStore
	}
)

func (controller *Controller) SetRoutes(eg *echo.Group)
