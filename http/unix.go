package http

import (
	"context"
	"net"
	"net/http"
)

func NewUnixSocketClient(path string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", path)
			},
		},
	}
}
