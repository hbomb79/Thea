package api

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/hbomb79/Thea/internal/api/controllers/auth"
	"github.com/hbomb79/Thea/internal/api/controllers/ingests"
	"github.com/hbomb79/Thea/internal/api/controllers/medias"
	"github.com/hbomb79/Thea/internal/api/controllers/targets"
	"github.com/hbomb79/Thea/internal/api/controllers/transcodes"
	"github.com/hbomb79/Thea/internal/api/controllers/users"
	"github.com/hbomb79/Thea/internal/api/controllers/workflows"
	"github.com/hbomb79/Thea/internal/api/gen"
	"github.com/hbomb79/Thea/internal/api/jwt"
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
		jwt.Store
	}

	TranscodeService interface {
		medias.TranscodeService
		transcodes.TranscodeService
	}

	// strictServerImpl offers an implementation of the generated
	// StrictServerInterface (generated by OpenAPI), which is
	// a union of all the methods exposed by the controllers
	strictServerImpl struct {
		*ingests.IngestsController
		*auth.AuthController
		*users.UserController
		*medias.MediaController
		*transcodes.TranscodesController
		*targets.TargetController
		*workflows.WorkflowController
	}

	// The RestGateway is a thin-wrapper around the Echo HTTP router. It's sole responsbility
	// is to create the routes Thea exposes, manage ongoing web socket connections and events,
	// and to enforce authc + authz middleware where applicable.
	RestGateway struct {
		*broadcaster
		config    *RestConfig
		ec        *echo.Echo
		socket    *websocket.SocketHub
		validator *validator.Validate
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
	// -- Setup JWT auth provider --
	apiBasePath := "/api/thea/v1"
	authKey, refreshKey, err := newJwtSigningKeys()
	if err != nil {
		panic(err)
	}
	authProvider := jwt.NewJwtAuth(store, fmt.Sprintf("%s/auth/", apiBasePath), authKey, refreshKey)

	// -- Setup Middleware --
	ec := echo.New()
	ec.OnAddRouteHandler = func(_ string, route echo.Route, _ echo.HandlerFunc, _ []echo.MiddlewareFunc) {
		log.Emit(logger.DEBUG, "Registered new route %s %s\n", route.Method, route.Path)
	}
	ec.HidePort = true
	ec.HideBanner = true
	ec.Pre(middleware.RemoveTrailingSlash())
	ec.Use(
		middleware.Recover(),
		middleware.LoggerWithConfig(middleware.LoggerConfig{
			Format: "[Request] ${time_rfc3339} :: ${method} ${uri} -> ${status} ${error} {ip=${remote_ip}, user_agent=${user_agent}}\n",
		}),
		middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: []string{"*"},
			// AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAccessControlAllowOrigin},
			// AllowMethods: []string{echo.OPTIONS, echo.GET, echo.HEAD, echo.PUT, echo.PATCH, echo.POST, echo.DELETE},
		}),
		authProvider.GetSecurityValidatorMiddleware(apiBasePath),
	)

	// -- Setup gateway --
	socket := websocket.New()
	gateway := &RestGateway{
		broadcaster: newBroadcaster(socket, ingestService, transcodeService, store),
		config:      config,
		ec:          ec,
		socket:      socket,
	}

	var serverImpl = gen.NewStrictHandler(&strictServerImpl{
		ingests.New(ingestService),
		auth.New(authProvider, store),
		users.NewController(store),
		medias.New(transcodeService, store),
		transcodes.New(transcodeService, store),
		targets.New(store),
		workflows.New(store),
	}, []gen.StrictMiddlewareFunc{requestBodyValidatorMiddleware})

	gen.RegisterHandlersWithBaseURL(ec, serverImpl, apiBasePath)
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
	refreshSecret, err := randomSecret(64) //512 bits
	if err != nil {
		return nil, nil, err
	}

	return authSecret, refreshSecret, nil
}

// Middleware to run Echo validator (see newValidator) against all incoming requests
func requestBodyValidatorMiddleware(f gen.StrictHandlerFunc, _ string) gen.StrictHandlerFunc {
	validator := newValidator()
	return func(ctx echo.Context, i interface{}) (interface{}, error) {
		if err := validator.Struct(i); err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("request body malformed: %s", err))
		}
		return f(ctx, i)
	}
}

// newValidator returns a validator which is used to validate the request
// body structs of all incoming requests. Any 'validate' tags on request
// structs (in the OpenAPI spec) must have their implementation here (excluding
// built-ins such as 'required').
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
