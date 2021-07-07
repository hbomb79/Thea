package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

type RouterOptions struct {
	ApiRoot string
	ApiPort int
	ApiHost string
}

type Router struct {
	Mux    *mux.Router
	routes []*routerListener
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
func (router *Router) CreateRoute(path string, handler http.HandlerFunc) *mux.Route {
	return router.Mux.HandleFunc(path, handler)
}

// Start accepts a struct of options and will use these options
// to configure the provided routes for the API endpoints, and
// will start the http listener.
func (router *Router) Start(opts *RouterOptions) error {
	if err := validateOpts(opts); err != nil {
		return err
	}

	err := http.ListenAndServe(fmt.Sprintf("%v:%v", opts.ApiHost, opts.ApiPort), router.Mux)
	if err != nil {
		return err
	}

	return nil
}

// validateOpts checks that the user provided options are valid
// so we can use them to configure our router
func validateOpts(opts *RouterOptions) error {
	if opts.ApiHost == "" || opts.ApiPort == 0 || opts.ApiRoot == "" {
		return errors.New("router options must contain ApiHost, ApiPort and ApiRoot to be used for routing.")
	}

	return nil
}
