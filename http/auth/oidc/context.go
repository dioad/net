package oidc

import (
	"context"

	"github.com/markbates/goth"
)

type oidcUserContext struct{}

func NewContextWithOIDCUserInfo(ctx context.Context, userInfo *goth.User) context.Context {
	return context.WithValue(ctx, oidcUserContext{}, userInfo)
}

func OIDCUserInfoFromContext(ctx context.Context) *goth.User {
	val := ctx.Value(oidcUserContext{})
	if val != nil {
		return val.(*goth.User)
	}
	return nil
}
