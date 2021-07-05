package router

import (
	"fmt"
	"strings"

	"github.com/gorilla/mux"
)

const (
	TPA_API_ROOT = "/api/%version%/"
)

type TpaRouterOptions struct {
	TpaApiVersion int
}

func NewTpaRouter(opts *TpaRouterOptions) *mux.Router {
	// TODO use this var
	_ = strings.ReplaceAll(TPA_API_ROOT, "%version%", fmt.Sprint(opts.TpaApiVersion))
	router := mux.NewRouter()

	return router
}
