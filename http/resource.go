package http

import (
	"github.com/gorilla/mux"
)

type Resource interface {
	RegisterRoutes(*mux.Router)
	Status() (interface{}, error)
}
