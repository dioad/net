package aws

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
)

// NewHTTPClient creates an HTTP client configured with AWS OIDC authentication.
// The client will automatically refresh AWS temporary credentials using the
// AssumeRoleWithWebIdentity API when they expire.
func NewHTTPClient(ctx context.Context, opts ...Opt) *http.Client {
	return oauth2.NewClient(ctx, NewTokenSource(opts...))
}
