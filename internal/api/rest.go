package api

import (
	"context"

	"github.com/hbomb79/Thea/internal/api/downloads"
	"github.com/hbomb79/Thea/internal/api/ingests"
	"github.com/hbomb79/Thea/internal/api/targets"
	"github.com/hbomb79/Thea/internal/api/transcodes"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type (
	RestConfig struct{}
	// The RestGateway is a thin-wrapper around the Echo HTTP router. It's sole responsbility
	// is to create the routes Thea exposes, and to enforce authc + authz middleware where applicable.
	RestGateway struct {
		ingestController    ingests.Controller
		transcodeController transcodes.Controller
		targetsController   targets.Controller
		downloadsController downloads.Controller
		ec                  *echo.Echo
	}
)

// NewRestGateway constructs the Echo router and populates it with all the
// routes defined by the various controllers. Each controller requires access
// to a data store, which are provided as arguments.
func NewRestGateway(config *RestConfig, ingestStore ingests.Store, transcodeStore transcodes.TranscodeStore) *RestGateway {
	ec := echo.New()
	gateway := &RestGateway{
		ec:                  ec,
		downloadsController: downloads.Controller{Store: nil},
		ingestController:    ingests.Controller{Store: ingestStore},
		transcodeController: transcodes.Controller{Store: transcodeStore},
		targetsController:   targets.Controller{Store: nil},
	}

	ec.Use(middleware.Logger())
	ec.Use(middleware.Recover())

	ingests := ec.Group("/api/thea/v1/ingests")
	gateway.ingestController.SetRoutes(ingests)

	transcodes := ec.Group("/api/thea/v1/transcodes")
	gateway.transcodeController.SetRoutes(transcodes)

	transcodeTargets := ec.Group("/api/thea/v1/transcodes-targets")
	gateway.targetsController.SetRoutes(transcodeTargets)

	downloads := ec.Group("/api/thea/v1/downloads")
	gateway.downloadsController.SetRoutes(downloads)

	return gateway
}

func (gateway *RestGateway) Run(ctx context.Context) {}
