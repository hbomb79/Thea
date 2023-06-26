package api

import (
	"github.com/hbomb79/Thea/internal/api/controllers"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type (
	// The RestGateway is a thin-wrapper around the Echo HTTP router. It's sole responsbility
	// is to create the routes Thea exposes, and to enforce authc + authz middleware where applicable.
	RestGateway struct {
		controllers.Ingests
		router *echo.Echo
	}
)

// NewRestGateway accepts all the dependencies it requires to correctly tie together
func NewRestGateway(ingestStore controllers.IngestStore) *RestGateway {
	ec := echo.New()
	gateway := &RestGateway{
		router:  ec,
		Ingests: controllers.Ingests{Store: ingestStore},
	}

	ec.Use(middleware.Logger())
	ec.Use(middleware.Recover())

	// JWT-based auth: TODO
	ingests := ec.Group("/api/thea/v1/ingests")
	gateway.Ingests.SetRoutes(ingests)

	return gateway
}

func (gateway *RestGateway) Start() {}
