// Package context provides utilities for managing authentication information in context.
package context

import "context"

type authenticatedPrincipal struct{}

// NewContextWithAuthenticatedPrincipal returns a new context with the provided principal ID.
func NewContextWithAuthenticatedPrincipal(ctx context.Context, principalId string) context.Context {
	return context.WithValue(ctx, authenticatedPrincipal{}, principalId)
}

// AuthenticatedPrincipalFromContext returns the principal ID from the provided context.
// It returns an empty string if no principal is found.
func AuthenticatedPrincipalFromContext(ctx context.Context) string {
	val := ctx.Value(authenticatedPrincipal{})
	if val != nil {
		return val.(string)
	}
	return ""
}
