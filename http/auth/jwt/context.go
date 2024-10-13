package jwt

import (
	"context"

	"github.com/markbates/goth"
)

type jwtUserContext struct{}

func NewContextWithClaims(ctx context.Context, userInfo *goth.User) context.Context {
	return context.WithValue(ctx, jwtUserContext{}, userInfo)
}

func ClaimsFromContext(ctx context.Context) *goth.User {
	val := ctx.Value(jwtUserContext{})
	if val != nil {
		return val.(*goth.User)
	}
	return nil
}
