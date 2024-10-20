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

		customClaims := claims.CustomClaims.(T)

		return customClaims
	}
	var emptyClaims T
	return emptyClaims
}
