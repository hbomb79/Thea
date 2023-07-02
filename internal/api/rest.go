package api

import (
	"context"

	"github.com/hbomb79/Thea/internal/api/downloads"
	"github.com/hbomb79/Thea/internal/api/ingests"
	"github.com/hbomb79/Thea/internal/api/lists"
	"github.com/hbomb79/Thea/internal/api/medias"
	"github.com/hbomb79/Thea/internal/api/targets"
	"github.com/hbomb79/Thea/internal/api/transcodes"
	"github.com/hbomb79/Thea/internal/api/workflows"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type (
	Controller interface {
		SetRoutes(*echo.Group)
	}

	RestConfig struct{}

	// The RestGateway is a thin-wrapper around the Echo HTTP router. It's sole responsbility
	// is to create the routes Thea exposes, and to enforce authc + authz middleware where applicable.
	RestGateway struct {
		ingestController    Controller
		transcodeController Controller
		targetsController   Controller
		downloadsController Controller
		workflowController  Controller
		listsController     Controller
		mediaController     Controller
		ec                  *echo.Echo
	}
)

// NewRestGateway constructs the Echo router and populates it with all the
// routes defined by the various controllers. Each controller requires access
// to a data store, which are provided as arguments.
func NewRestGateway(config *RestConfig, ingestStore ingests.Store, transcodeStore transcodes.Store) *RestGateway {
	ec := echo.New()
	gateway := &RestGateway{
		ec:                  ec,
		downloadsController: downloads.New(nil),
		ingestController:    ingests.New(ingestStore),
		transcodeController: transcodes.New(transcodeStore),
		targetsController:   targets.New(nil),
		workflowController:  workflows.New(nil),
		mediaController:     medias.New(nil),
		listsController:     lists.New(nil),
	}

	ec.Use(middleware.Logger())
	ec.Use(middleware.Recover())
	ec.Use(middleware.AddTrailingSlash())

	ingests := ec.Group("/api/thea/v1/ingests")
	gateway.ingestController.SetRoutes(ingests)

	transcodes := ec.Group("/api/thea/v1/transcodes")
	gateway.transcodeController.SetRoutes(transcodes)

	transcodeTargets := ec.Group("/api/thea/v1/transcode-targets")
	gateway.targetsController.SetRoutes(transcodeTargets)

	transcodeWorkflows := ec.Group("/api/thea/v1/transcode-workflows")
	gateway.workflowController.SetRoutes(transcodeWorkflows)

	downloads := ec.Group("/api/thea/v1/downloads")
	gateway.downloadsController.SetRoutes(downloads)

	media := ec.Group("/api/thea/v1/media")
	gateway.mediaController.SetRoutes(media)

	lists := ec.Group("/api/thea/v1/lists")
	gateway.listsController.SetRoutes(lists)

	return gateway
}

func (gateway *RestGateway) Run(ctx context.Context) {}
