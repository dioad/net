package auth

import (
	"context"
	"net/http"
)

type authenticatedPrincipal struct{}

type ClientAuth interface {
	AddAuth(*http.Request) error
}

func NewContextWithAuthenticatedPrincipal(ctx context.Context, principalId string) context.Context {
	return context.WithValue(ctx, authenticatedPrincipal{}, principalId)
}

func AuthenticatedPrincipalFromContext(ctx context.Context) string {
	val := ctx.Value(authenticatedPrincipal{})
	if val != nil {
		return val.(string)
	}
	return ""
}
