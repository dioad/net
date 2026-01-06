package jwt

import (
	"context"

	jwtmiddleware "github.com/auth0/go-jwt-middleware/v2"
)

type jwtUserContext struct{}

// func NewContextWithClaims(ctx context.Context, userInfo *goth.User) context.Context {
// 	return context.WithValue(ctx, jwtUserContext{}, userInfo)
// }
//
// func ClaimsFromContext(ctx context.Context) *goth.User {
// 	val := ctx.Value(jwtUserContext{})
// 	if val != nil {
// 		return val.(*goth.User)
// 	}
// 	return nil
// }

func TokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(jwtmiddleware.ContextKey{}).(string)
	return token, ok
}
