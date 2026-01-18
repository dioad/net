package oidc

import (
	"context"

	"github.com/markbates/goth"
)

type oidcUserContext struct{}

// NewContextWithOIDCUserInfo returns a new context with the provided OIDC user info.
func NewContextWithOIDCUserInfo(ctx context.Context, userInfo *goth.User) context.Context {
	return context.WithValue(ctx, oidcUserContext{}, userInfo)
}

// OIDCUserInfoFromContext returns the OIDC user info from the provided context.
// It returns nil if no user info is found.
func OIDCUserInfoFromContext(ctx context.Context) *goth.User {
	val := ctx.Value(oidcUserContext{})
	if val != nil {
		return val.(*goth.User)
	}
	return nil
}

type authTokenContext struct{}

// ContextWithAccessToken returns a new context with the provided access token.
func ContextWithAccessToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, authTokenContext{}, token)
}

// AccessTokenFromContext returns the access token from the provided context.
// It returns an empty string if no token is found.
func AccessTokenFromContext(ctx context.Context) string {
	val := ctx.Value(authTokenContext{})
	if val != nil {
		if token, ok := val.(string); ok {
			return token
		}
	}
	return ""
}
