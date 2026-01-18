package http

import (
	"net/http"

	"github.com/gorilla/mux"
)

// Resource is a marker interface for HTTP resources.
type Resource interface{}

// StatusResource is an interface for resources that can report their status.
type StatusResource interface {
	Status() (interface{}, error)
}

// UseResource is an interface for resources that support applying middleware.
type UseResource interface {
	Use(...mux.MiddlewareFunc) UseResource
}

// DefaultResource is an interface for resources that can register their routes on a router.
type DefaultResource interface {
	RegisterRoutes(*mux.Router)
}

// PathResource is an interface for resources that can register their routes on a router with a path prefix.
type PathResource interface {
	RegisterRoutesWithPrefix(*mux.Router, string)
}

// RootResource is an interface for the root resource of the server.
type RootResource interface {
	// Resource
	Index() http.HandlerFunc
}
