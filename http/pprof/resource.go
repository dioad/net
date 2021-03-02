package pprof

import (
	"net/http/pprof"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

type Resource struct {
	Logger zerolog.Logger
}

type Status struct {
	Status string
}

// RegisterRoutes ...
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

func (dr *Resource) Status() (interface{}, error) {
	return Status{Status: "OK"}, nil
}

func NewResource(logger zerolog.Logger) *Resource {
	return &Resource{
		Logger: logger,
	}
}
