package api

import (
	"context"
	"regexp"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/hbomb79/Thea/internal/api/ingests"
	"github.com/hbomb79/Thea/internal/api/medias"
	"github.com/hbomb79/Thea/internal/api/targets"
	"github.com/hbomb79/Thea/internal/api/transcodes"
	"github.com/hbomb79/Thea/internal/api/workflows"
	"github.com/hbomb79/Thea/internal/http/websocket"
	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const (
	alphaNumericWhitespaceRegexString = "^[a-zA-Z0-9\\s]+$"
)

var (
	log                         = logger.Get("API")
	alphaNumericWhitespaceRegex = regexp.MustCompile(alphaNumericWhitespaceRegexString)
)

type (
	RestConfig struct {
		HostAddr string `toml:"host_address" env:"API_HOST_ADDR" env-default:"0.0.0.0:8080"`
	}

	Controller interface {
		SetRoutes(*echo.Group)
	}

	// Store represents a union of all the controller store requirements, typically
	// fulfilled by Thea's store orchestrator
	Store interface {
		targets.Store
		workflows.Store
		transcodes.Store
		medias.Store
	}

	TranscodeService interface {
		medias.TranscodeService
		transcodes.TranscodeService
	}

	// The RestGateway is a thin-wrapper around the Echo HTTP router. It's sole responsbility
	// is to create the routes Thea exposes, manage ongoing web socket connections and events,
	// and to enforce authc + authz middleware where applicable.
	RestGateway struct {
		*broadcaster
		config              *RestConfig
		ec                  *echo.Echo
		socket              *websocket.SocketHub
		ingestController    Controller
		transcodeController Controller
		targetsController   Controller
		workflowController  Controller
		mediaController     Controller
	}
)

// NewRestGateway constructs the Echo router and populates it with all the
// routes defined by the various controllers. Each controller requires access
// to a data store, which are provided as arguments.
func NewRestGateway(
	config *RestConfig,
	ingestService ingests.IngestService,
	transcodeService TranscodeService,
	store Store,
) *RestGateway {
	ec := echo.New()
	ec.OnAddRouteHandler = func(_ string, route echo.Route, _ echo.HandlerFunc, _ []echo.MiddlewareFunc) {
		log.Emit(logger.DEBUG, "Registered new route %s %s\n", route.Method, route.Path)
	}
	ec.HidePort = true
	ec.HideBanner = true

	validate := newValidator()
	socket := websocket.New()
	gateway := &RestGateway{
		broadcaster:         newBroadcaster(socket, ingestService, transcodeService, store),
		config:              config,
		ec:                  ec,
		socket:              socket,
		ingestController:    ingests.New(validate, ingestService),
		transcodeController: transcodes.New(validate, transcodeService, store),
		targetsController:   targets.New(validate, store),
		workflowController:  workflows.New(validate, store),
		mediaController:     medias.New(validate, transcodeService, store),
	}

	ec.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "[Request] ${time_rfc3339} :: ${method} ${uri} -> ${status} ${error} {ip=${remote_ip}, user_agent=${user_agent}}\n",
	}))
	ec.Use(middleware.Recover())
	ec.Pre(middleware.AddTrailingSlash())

	ec.GET("/api/thea/v1/activity/ws/", func(ec echo.Context) error {
		gateway.socket.UpgradeToSocket(ec.Response(), ec.Request())
		return nil
	})

	ingests := ec.Group("/api/thea/v1/ingests")
	gateway.ingestController.SetRoutes(ingests)

	transcodes := ec.Group("/api/thea/v1/transcodes")
	gateway.transcodeController.SetRoutes(transcodes)

	transcodeTargets := ec.Group("/api/thea/v1/transcode-targets")
	gateway.targetsController.SetRoutes(transcodeTargets)

	transcodeWorkflows := ec.Group("/api/thea/v1/transcode-workflows")
	gateway.workflowController.SetRoutes(transcodeWorkflows)

	media := ec.Group("/api/thea/v1/media")
	gateway.mediaController.SetRoutes(media)

	return gateway
}

func (gateway *RestGateway) Run(parentCtx context.Context) error {
	ctx, ctxCancel := context.WithCancelCause(parentCtx)
	wg := &sync.WaitGroup{}

	// Start echo router
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Emit(logger.NEW, "Started HTTP router at %s\n", gateway.config.HostAddr)
		if err := gateway.ec.Start(gateway.config.HostAddr); err != nil {
			ctxCancel(err)
		}
	}()

	// Start thread to listen for context cancellation
	go func(ec *echo.Echo) {
		<-ctx.Done()
		ec.Close()
	}(gateway.ec)

	// Start websocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		gateway.socket.Start(ctx)
	}()

	wg.Wait()

	// Return cancellation cause if any, otherwise nil as parent context
	// cancellation is not an error case we should report.
	if cause := context.Cause(ctx); cause != ctx.Err() {
		return cause
	}

	return nil
}

func newValidator() *validator.Validate {
	validate := validator.New()
	validate.RegisterValidation("alphaNumericWhitespaceTrimmed", func(fl validator.FieldLevel) bool {
		str := fl.Field().String()
		if len(strings.TrimSpace(str)) != len(str) {
			return false
		}

		return alphaNumericWhitespaceRegex.MatchString(str)
	}, true)

	return validate
}
