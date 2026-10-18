package http

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type readinessOnly struct{ err error }

func (r readinessOnly) Ready(context.Context) (any, error) { return "checked", r.err }

type statusOnly struct{}

func (statusOnly) Status(context.Context) (any, error) { return map[string]int{"tunnels": 2}, nil }

func newHealthServer(t *testing.T) *Server {
	t.Helper()
	return NewServer(Config{EnableHealth: true, EnableStatus: true}, WithLogger(zerolog.Nop()))
}

func TestAddHealthSource_ReportsReadinessWithoutMountingARoute(t *testing.T) {
	t.Parallel()
	s := newHealthServer(t)
	require.NoError(t, s.AddHealthSource("gate", readinessOnly{err: errors.New("draining")}))
	addr := serveOn(t, s)

	status, body := getPath(t, addr, "/health/ready")
	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.Contains(t, body, `"gate":"draining"`)

	status, _ = getPath(t, addr, "/gate")
	assert.Equal(t, http.StatusNotFound, status, "a health source has no route of its own")
}

func TestAddHealthSource_ReportsStatus(t *testing.T) {
	t.Parallel()
	s := newHealthServer(t)
	require.NoError(t, s.AddHealthSource("tunnels", statusOnly{}))
	addr := serveOn(t, s)

	status, body := getPath(t, addr, "/status")

	assert.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, `"tunnels":{"tunnels":2}`)
}

func TestAddHealthSource_RejectsAValueWithNoHealthMethods(t *testing.T) {
	t.Parallel()
	s := newHealthServer(t)

	err := s.AddHealthSource("nothing", struct{}{})

	require.ErrorIs(t, err, ErrNotAHealthSource)
}

func TestAddHealthSource_OnThePrimaryIsReportedByItsOperationalServer(t *testing.T) {
	t.Parallel()
	primary := newPrimary()
	ops := NewOperationalServer(primary, OperationalConfig{}, WithLogger(zerolog.Nop()))
	require.NoError(t, primary.AddHealthSource("gate", readinessOnly{err: errors.New("not started")}))
	addr := serveOn(t, ops)

	status, body := getPath(t, addr, "/health/ready")

	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.Contains(t, body, "not started")
}

func TestHealthRegistry_RegisteringWhileServingIsSafe(t *testing.T) {
	t.Parallel()
	s := newHealthServer(t)
	addr := serveOn(t, s)

	var wg sync.WaitGroup
	wg.Go(func() {
		for i := range 50 {
			_ = s.AddHealthSource("source-"+strconv.Itoa(i), readinessOnly{})
		}
	})
	wg.Go(func() {
		for range 20 {
			getPath(t, addr, "/health/ready")
		}
	})
	wg.Wait()

	status, _ := getPath(t, addr, "/health/ready")
	assert.Equal(t, http.StatusOK, status)
}

func TestNewOperationalServer_ServesOverTLS(t *testing.T) {
	t.Parallel()
	// Borrow a test certificate, and a client that trusts it, from httptest.
	certSource := httptest.NewUnstartedServer(http.NotFoundHandler())
	certSource.StartTLS()
	t.Cleanup(certSource.Close)

	ops := NewOperationalServer(newPrimary(), OperationalConfig{TLSConfig: certSource.TLS.Clone()}, WithLogger(zerolog.Nop()))
	addr := serveOn(t, ops)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://"+addr+"/health/ready", nil)
	require.NoError(t, err)
	resp, err := certSource.Client().Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// rawGet sends prefix then GET path over a fresh connection and returns the
// response status, or an error if the server closed the connection.
func rawGet(t *testing.T, addr, prefix, path string) (int, error) {
	t.Helper()
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(t.Context(), "tcp", addr)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
	_, err = io.WriteString(conn, prefix+"GET "+path+" HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n")
	require.NoError(t, err)
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode, nil
}

func TestNewOperationalServer_ProxyProtocolIsOffUnlessConfigured(t *testing.T) {
	t.Parallel()

	plain := serveOn(t, NewOperationalServer(newPrimary(), OperationalConfig{}, WithLogger(zerolog.Nop())))
	status, err := rawGet(t, plain, "", "/health/ready")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, status, "a direct probe needs no PROXY header by default")

	proxied := serveOn(t, NewOperationalServer(newPrimary(), OperationalConfig{EnableProxyProtocol: true}, WithLogger(zerolog.Nop())))
	_, err = rawGet(t, proxied, "", "/health/ready")
	require.Error(t, err, "with PROXY protocol on (require), a connection without a header is refused")
	status, err = rawGet(t, proxied, proxyHeader, "/health/ready")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)
}
