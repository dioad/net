// Package pprof provides an HTTP resource for exposing pprof debugging endpoints.
package pprof

import (
	"net/http"
	"net/http/pprof"

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

// Handler returns the HTTP handler containing the pprof endpoints.
func (dr *Resource) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", pprof.Index)
	mux.HandleFunc("/cmdline", pprof.Cmdline)
	mux.HandleFunc("/profile", pprof.Profile)
	mux.HandleFunc("/symbol", pprof.Symbol)

	// Manually add support for paths linked to by index page at /debug/pprof/
	mux.Handle("/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/heap", pprof.Handler("heap"))
	mux.Handle("/threadcreate", pprof.Handler("threadcreate"))
	mux.Handle("/block", pprof.Handler("block"))
	mux.Handle("/trace", pprof.Handler("trace"))
	mux.Handle("/mutex", pprof.Handler("mutex"))

	return mux
}

// Status returns the status of the pprof resource.
func (dr *Resource) Status() (any, error) {
	return Status{Status: "OK"}, nil
}

// NewResource creates a new pprof resource.
func NewResource(logger zerolog.Logger) *Resource {
	return &Resource{
		Logger: logger,
	}
}
