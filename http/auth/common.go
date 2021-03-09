package auth

import "net/http"

type AuthenticatedPrincipal struct{}

type ClientAuth interface {
	AddAuth(*http.Request) error
}
