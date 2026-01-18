// Package pprof provides an HTTP resource for exposing pprof debugging endpoints.
package pprof

import (
	"net/http/pprof"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// Resource implements the Resource interface for pprof endpoints.
type Resource struct {
	Logger zerolog.Logger
}

// Status represents the status of the pprof resource.
type Status struct {
	Status string
}

// RegisterRoutes registers the pprof endpoints on the provided router.
func (dr *Resource) RegisterRoutes(parentRouter *mux.Router) {
	parentRouter.HandleFunc("/", pprof.Index)
	parentRouter.HandleFunc("/cmdline", pprof.Cmdline)
	parentRouter.HandleFunc("/profile", pprof.Profile)
	parentRouter.HandleFunc("/symbol", pprof.Symbol)

	// Manually add support for paths linked to by index page at /debug/pprof/
	parentRouter.Handle("/goroutine", pprof.Handler("goroutine"))
	parentRouter.Handle("/heap", pprof.Handler("heap"))
	parentRouter.Handle("/threadcreate", pprof.Handler("threadcreate"))
	parentRouter.Handle("/block", pprof.Handler("block"))
	parentRouter.Handle("/trace", pprof.Handler("trace"))
	parentRouter.Handle("/mutex", pprof.Handler("mutex"))
}

// Status returns the status of the pprof resource.
func (dr *Resource) Status() (interface{}, error) {
	return Status{Status: "OK"}, nil
}

// NewResource creates a new pprof resource.
func NewResource(logger zerolog.Logger) *Resource {
	return &Resource{
		Logger: logger,
	}
}
