package resource

import (
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/dioad/net/http/auth/oidc"
)

// SessionResource is an HTTP resource that manages authentication sessions.
type SessionResource struct {
	AuthHandler *oidc.Handler
	Logger      zerolog.Logger
}

// SessionResourceStatus represents the status of the session resource.
type SessionResourceStatus struct {
	Status string
}

// RegisterRoutes registers the session resource routes on the provided router.
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

// Status returns the status of the session resource.
func (dr *SessionResource) Status() (any, error) {
	return SessionResourceStatus{
		Status: "OK",
	}, nil
}

// NewSessionResource creates a new session resource with the provided OIDC handler and logger.
func NewSessionResource(handler *oidc.Handler, logger zerolog.Logger) *SessionResource {
	return &SessionResource{
		AuthHandler: handler,
		Logger:      logger,
	}
}
