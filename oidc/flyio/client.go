package flyio

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
)

func NewHTTPClient(context context.Context, opts ...Opt) *http.Client {
	return oauth2.NewClient(context, NewTokenSource(opts...))
}
