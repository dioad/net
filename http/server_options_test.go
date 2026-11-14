package http

import (
	"net/http"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerOptions_TurnOnOperationalEndpoints(t *testing.T) {
	t.Parallel()

	paths := []string{"/metrics", "/debug/pprof/cmdline", "/status", "/health/live", "/health/ready"}
	tests := []struct {
		name   string
		option ServerOption
		served []string
	}{
		{"WithPrometheusMetrics", WithPrometheusMetrics(), []string{"/metrics"}},
		{"WithDebug", WithDebug(), []string{"/debug/pprof/cmdline"}},
		{"WithStatus", WithStatus(), []string{"/status"}},
		{"WithHealth", WithHealth(), []string{"/health/live", "/health/ready"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			addr := serveOn(t, NewServer(Config{}, WithLogger(zerolog.Nop()), tt.option))

			for _, path := range paths {
				want := http.StatusNotFound
				for _, served := range tt.served {
					if served == path {
						want = http.StatusOK
					}
				}
				status, _ := getPath(t, addr, path)
				assert.Equal(t, want, status, path)
			}
		})
	}
}

func TestNewServer_ServesNoOperationalEndpointsByDefault(t *testing.T) {
	t.Parallel()
	addr := serveOn(t, NewServer(Config{}, WithLogger(zerolog.Nop())))

	for _, path := range []string{"/metrics", "/debug/pprof/cmdline", "/status", "/health/live", "/health/ready"} {
		status, _ := getPath(t, addr, path)
		assert.Equal(t, http.StatusNotFound, status, path)
	}
}

func TestWithProxyProtocol_AppliesThePolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		policy        ProxyProtocolPolicy
		withoutHeader bool // whether a connection without a header is answered
	}{
		{"require", ProxyProtocolRequire, false},
		{"optional", ProxyProtocolOptional, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := NewServer(Config{}, WithLogger(zerolog.Nop()), WithProxyProtocol(tt.policy))
			server.AddHandlerFunc("GET /addr", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set(remoteHostHeader, r.RemoteAddr)
				w.WriteHeader(http.StatusNoContent)
			})
			addr := serveOn(t, server)

			got, err := requestAddr(t, addr, proxyHeader)
			require.NoError(t, err)
			assert.Equal(t, "203.0.113.7:40000", got, "the handler sees the address from the header")

			_, err = requestAddr(t, addr, "")
			if tt.withoutHeader {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err, "a connection without a PROXY header is refused")
			}
		})
	}
}
