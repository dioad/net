package http

import (
	"net/http"

	"github.com/gorilla/mux"
)

type Resource interface{}

type StatusResource interface {
	Status() (interface{}, error)
}

type DefaultResource interface {
	RegisterRoutes(*mux.Router)
}
type PathResource interface {
	RegisterRoutesWithPrefix(*mux.Router, string)
}

type RootResource interface {
	//Resource
	Index() http.HandlerFunc
}
