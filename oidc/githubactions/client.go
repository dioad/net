package githubactions

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
)

// NewHTTPClient creates an HTTP client configured with GitHub Actions OIDC authentication
func NewHTTPClient(ctx context.Context, opts ...Opt) *http.Client {
	return oauth2.NewClient(ctx, NewTokenSource(opts...))
}
