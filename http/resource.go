package http

import (
	"net/http"
)

// Resource is a marker interface for HTTP resources.
// It mandates that a resource must provide an http.Handler that can be mounted
// onto the server's main multiplexer.
type Resource interface {
	Handler() http.Handler
}

// Middleware is a standard function signature for HTTP middleware.
type Middleware func(http.Handler) http.Handler

// Chain builds an http.Handler composed of the target handler and the provided middlewares.
// The middlewares are applied in reverse order so the first middleware in the list
// is the first to execute.
func Chain(handler http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i] != nil {
			handler = middlewares[i](handler)
		}
	}
	return handler
}

// StatusResource is an interface for resources that can report their status.
type StatusResource interface {
	Status() (any, error)
}

// LivenessResource is an interface for resources that can report their liveness.
type LivenessResource interface {
	Live() error
}

// ReadinessResource is an interface for resources that can report their readiness.
type ReadinessResource interface {
	Ready() (any, error)
}

// RootResource is an interface for the root resource of the server.
type RootResource interface {
	// Resource
	Index() http.HandlerFunc
}
