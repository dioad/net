package resource

import (
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/dioad/net/http/auth/oidc"
)

type SessionResource struct {
	AuthHandler *oidc.Handler
	Logger      zerolog.Logger
}

type SessionResourceStatus struct {
	Status string
}

// RegisterRoutes ...
func (dr *SessionResource) RegisterRoutes(parentRouter *mux.Router) {

	logoutHandler := dr.AuthHandler.LogoutHandler
	// authHandler := dr.AuthHandler.AuthWrapper
	callbackHandler := dr.AuthHandler.Callback
	authStartHandler := dr.AuthHandler.AuthStart

	// `provider` path parameter is required by the gothic library
	// parentRouter.HandleFunc("/auth/{provider}/logout", dr.LogoutGet()).Methods("GET")
	parentRouter.HandleFunc("/logout", logoutHandler()).Methods("GET")
	parentRouter.HandleFunc("/auth/{provider}/callback", callbackHandler()).Methods("GET")
	parentRouter.HandleFunc("/auth/{provider}", authStartHandler()).Methods("GET")
}

func (dr *SessionResource) Status() (interface{}, error) {
	return SessionResourceStatus{
		Status: "OK",
	}, nil
}

func NewSessionResource(handler *oidc.Handler, logger zerolog.Logger) *SessionResource {
	return &SessionResource{
		AuthHandler: handler,
		Logger:      logger,
	}
}
