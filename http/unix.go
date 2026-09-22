package http

import (
	"context"
	"net"
	"net/http"
)

// NewUnixSocketClient returns an *http.Client that dials the Unix domain
// socket at path for every request, ignoring the request's own network/address.
func NewUnixSocketClient(path string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer

				return d.DialContext(ctx, "unix", path)
			},
		},
	}
}
