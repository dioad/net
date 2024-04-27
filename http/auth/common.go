package auth

import (
	stdctx "context"
	"net/http"
)

type ClientAuth interface {
	AddAuth(*http.Request) error
}

type Middleware interface {
	//	AuthRequest(r *http.Request) (stdctx.Context, error)
	Wrap(http.Handler) http.Handler
}

type Authenticator interface {
	AuthRequest(r *http.Request) (stdctx.Context, error)
	// Wrap(http.Handler) http.Handler
}
