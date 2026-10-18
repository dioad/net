package http

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startProxyProtocolServer serves a handler that echoes the remote address it
// sees, with PROXY protocol enabled under policy, and returns its address.
func startProxyProtocolServer(t *testing.T, policy ProxyProtocolPolicy) string {
	t.Helper()

	server := NewServer(Config{EnableProxyProtocol: true, ProxyProtocolPolicy: policy}, WithLogger(zerolog.Nop()))
	server.AddHandlerFunc("GET /addr", func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		w.Header().Set(remoteHostHeader, host)
		w.WriteHeader(http.StatusNoContent)
	})

	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = server.Serve(ln) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	})
	return ln.Addr().String()
}

// remoteHostHeader carries the remote host the handler saw.
const remoteHostHeader = "X-Remote-Host"

// requestAddr sends GET /addr over a fresh connection, preceded by header
// when it is not empty, and returns the remote host the handler saw, or an
// error if the server closed the connection without answering.
func requestAddr(t *testing.T, addr, header string) (string, error) {
	t.Helper()

	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(t.Context(), "tcp", addr)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))

	_, err = io.WriteString(conn, header+"GET /addr HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n")
	require.NoError(t, err)

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.Header.Get(remoteHostHeader), nil
}

const proxyHeader = "PROXY TCP4 203.0.113.7 192.0.2.1 40000 443\r\n"

func TestServe_ProxyProtocolPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		policy         ProxyProtocolPolicy
		withoutHeader  bool // whether a connection without a header is answered
		headerAddrSeen string
	}{
		{"unset requires a header", "", false, "203.0.113.7"},
		{"require", ProxyProtocolRequire, false, "203.0.113.7"},
		{"optional", ProxyProtocolOptional, true, "203.0.113.7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			addr := startProxyProtocolServer(t, tt.policy)

			got, err := requestAddr(t, addr, proxyHeader)
			require.NoError(t, err, "a connection with a PROXY header is always answered")
			assert.Equal(t, tt.headerAddrSeen, got, "the handler sees the address from the header")

			got, err = requestAddr(t, addr, "")
			if tt.withoutHeader {
				require.NoError(t, err)
				assert.Equal(t, "127.0.0.1", got, "without a header the handler sees the peer")
			} else {
				assert.Error(t, err, "a connection without a PROXY header is refused")
			}
		})
	}
}

func TestServe_RejectsAnUnknownProxyProtocolPolicy(t *testing.T) {
	t.Parallel()

	server := NewServer(Config{EnableProxyProtocol: true, ProxyProtocolPolicy: "sometimes"}, WithLogger(zerolog.Nop()))
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	err = server.Serve(ln)

	require.ErrorContains(t, err, `unknown proxy protocol policy "sometimes"`)
}
