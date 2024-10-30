package oidc

import (
	"context"

	jwtmiddleware "github.com/auth0/go-jwt-middleware/v2"
	jwtvalidator "github.com/auth0/go-jwt-middleware/v2/validator"
)

func CustomClaimsFromContext[T jwtvalidator.CustomClaims](ctx context.Context) T {
	val := ctx.Value(jwtmiddleware.ContextKey{})
	if val != nil {
		claims := val.(*jwtvalidator.ValidatedClaims)

		customClaims, ok := claims.CustomClaims.(T)

		if ok {
			return customClaims
		}
	}
	var emptyClaims T
	return emptyClaims
}
