package api

import (
	"context"
	"crypto/rand"
	"regexp"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/hbomb79/Thea/internal/api/auth"
	"github.com/hbomb79/Thea/internal/api/ingests"
	"github.com/hbomb79/Thea/internal/api/medias"
	"github.com/hbomb79/Thea/internal/api/targets"
	"github.com/hbomb79/Thea/internal/api/transcodes"
	"github.com/hbomb79/Thea/internal/api/users"
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
		auth.Store
		users.Store
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
		userController      Controller
		authController      Controller
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

	authKey, refreshKey, err := newJwtSigningKeys()
	if err != nil {
		panic(err)
	}

	authProvider := NewJwtAuth(store, "/api/thea/v1/auth/", authKey, refreshKey)

	gateway := &RestGateway{
		broadcaster:         newBroadcaster(socket, ingestService, transcodeService, store),
		config:              config,
		ec:                  ec,
		socket:              socket,
		ingestController:    ingests.New(authProvider, validate, ingestService),
		transcodeController: transcodes.New(authProvider, validate, transcodeService, store),
		targetsController:   targets.New(authProvider, validate, store),
		workflowController:  workflows.New(authProvider, validate, store),
		mediaController:     medias.New(authProvider, validate, transcodeService, store),
		userController:      users.NewController(authProvider, store),
		authController:      auth.New(authProvider, store),
	}

	ec.Pre(middleware.AddTrailingSlash())
	ec.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "[Request] ${time_rfc3339} :: ${method} ${uri} -> ${status} ${error} {ip=${remote_ip}, user_agent=${user_agent}}\n",
	}))
	ec.Use(middleware.Recover())

	auth := ec.Group("/api/thea/v1/auth")
	gateway.authController.SetRoutes(auth)

	// NB: this middleware must come before any other attempt
	// to access the user token (including other middleware)
	// as it populates the token in the request context!
	authenticated := authProvider.GetAuthenticatedMiddleware()

	ec.GET("/api/thea/v1/activity/ws/", func(ec echo.Context) error {
		gateway.socket.UpgradeToSocket(ec.Response(), ec.Request())
		return nil
	}, authenticated)

	ingests := ec.Group("/api/thea/v1/ingests", authenticated)
	gateway.ingestController.SetRoutes(ingests)

	transcodes := ec.Group("/api/thea/v1/transcodes", authenticated)
	gateway.transcodeController.SetRoutes(transcodes)

	transcodeTargets := ec.Group("/api/thea/v1/transcode-targets", authenticated)
	gateway.targetsController.SetRoutes(transcodeTargets)

	transcodeWorkflows := ec.Group("/api/thea/v1/transcode-workflows", authenticated)
	gateway.workflowController.SetRoutes(transcodeWorkflows)

	media := ec.Group("/api/thea/v1/media", authenticated)
	gateway.mediaController.SetRoutes(media)

	users := ec.Group("/api/thea/v1/users", authenticated)
	gateway.userController.SetRoutes(users)

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

func newJwtSigningKeys() ([]byte, []byte, error) {
	authSecret, err := randomSecret(64) //512 bits
	if err != nil {
		return nil, nil, err
	}
	refreshSecret, err := randomSecret(64) //512 bbits
	if err != nil {
		return nil, nil, err
	}

	return authSecret, refreshSecret, nil
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

// randomSecret generates a random byte slice of the
// requested length. This is used to create cryptographically
// secure random byte arrays for use with JWT signing.
func randomSecret(length uint32) ([]byte, error) {
	secret := make([]byte, length)
	_, err := rand.Read(secret)
	if err != nil {
		return nil, err
	}

	return secret, nil
}
