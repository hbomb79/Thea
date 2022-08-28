package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/hbomb79/TPA/pkg/logger"
)

var log = logger.Get("HTTP")

type RouterOptions struct {
	ApiRoot string
	ApiPort int
	ApiHost string
}

type Router struct {
	Mux    *mux.Router
	routes []*routerListener
	server *http.Server
}

type routerListener struct {
	path    string
	methods []string
	handler http.HandlerFunc
}

// NewRouter creates a new Router struct and creates the mux router and the
// slice of routes before returning the *Router
func NewRouter() *Router {
	return &Router{
		Mux:    mux.NewRouter(),
		routes: make([]*routerListener, 0),
	}
}

// CreateRoute will register a new listener on the provided path
// after prepending it with the API root we're using or this particular
// router - this allows us to change the location of the API without
// having to adjust every single handler.
func (router *Router) CreateRoute(path string, methods string, handler http.HandlerFunc) {
	// Remove any whitespace so we can split on ',' to form
	// a slice without leading/trailing whitespace
	methods = strings.ReplaceAll(methods, " ", "")

	router.routes = append(router.routes, &routerListener{path, strings.Split(methods, ","), handler})
}

// Start accepts a struct of options and will use these options
// to configure the provided routes for the API endpoints, and
// will start the http listener.
func (router *Router) Start(opts *RouterOptions) error {
	if err := validateOpts(opts); err != nil {
		return err
	}

	log.Emit(logger.NEW, "Starting HTTP router\n")
	router.buildRoutes(opts)

	host := fmt.Sprintf("%v:%v", opts.ApiHost, opts.ApiPort)
	router.server = &http.Server{Addr: host, Handler: trimTrailingSlashesMiddleware(router.Mux)}
	if err := router.server.ListenAndServe(); err != nil {
		return err
	}

	return nil
}

func (router *Router) Stop() {
	if router.server == nil {
		log.Emit(logger.WARNING, "HTTP Router is already closed!\n")
		return
	}

	log.Emit(logger.STOP, "Closing HTTP router\n")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	router.server.Shutdown(ctx)
}

// buildRoutes is used internally to take the list of routes
// created by the user (Router.CreateRoute) and adds them to the *mux.Router
// on this Router - in the process, paths are prepended with the
// root address of this API (opts.ApiRoot) and duplicate slashes
// are removed.
func (router *Router) buildRoutes(opts *RouterOptions) {
	for _, route := range router.routes {
		routePath := strings.ReplaceAll(fmt.Sprintf("%s/%s", opts.ApiRoot, route.path), "//", "/")
		log.Emit(logger.NEW, "Building Mux route %v %v\n", routePath, route.methods)

		muxRoute := router.Mux.HandleFunc(routePath, route.handler)
		if len(route.methods) > 0 {
			muxRoute = muxRoute.Methods(route.methods...)
		}
	}
}

// validateOpts checks that the user provided options are valid
// so we can use them to configure our router
func validateOpts(opts *RouterOptions) error {
	if opts.ApiHost == "" || opts.ApiPort == 0 || opts.ApiRoot == "" {
		return errors.New("router options must contain ApiHost, ApiPort and ApiRoot to be used for routing.")
	}

	return nil
}
