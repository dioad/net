package http

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serveOn starts s on a loopback port and returns its address.
func serveOn(t *testing.T, s *Server) string {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = s.Serve(ln) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	})
	return ln.Addr().String()
}

func getPath(t *testing.T, addr, path string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+addr+path, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

// requireCredentials rejects every request without an Authorization header,
// standing in for a primary server's authentication.
func requireCredentials(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// newPrimary returns a server with health and an authenticated API route, as
// a service's main listener would have.
func newPrimary() *Server {
	primary := NewServer(Config{EnableHealth: true}, WithLogger(zerolog.Nop()))
	primary.Use(requireCredentials)
	primary.AddHandlerFunc("GET /api/things", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return primary
}

type readyResource struct{ err error }

func (r readyResource) Handler() http.Handler              { return http.NotFoundHandler() }
func (r readyResource) Ready(context.Context) (any, error) { return "checked", r.err }

func TestNewOperationalServer_ServesHealthWithoutThePrimarysAuthentication(t *testing.T) {
	t.Parallel()
	primary := newPrimary()
	ops := NewOperationalServer(primary, OperationalConfig{}, WithLogger(zerolog.Nop()))
	primaryAddr, opsAddr := serveOn(t, primary), serveOn(t, ops)

	status, _ := getPath(t, primaryAddr, "/health/ready")
	assert.Equal(t, http.StatusUnauthorized, status, "the primary keeps its authentication")

	status, _ = getPath(t, opsAddr, "/health/ready")
	assert.Equal(t, http.StatusOK, status, "a platform check reaches readiness without credentials")
	status, _ = getPath(t, opsAddr, "/health/live")
	assert.Equal(t, http.StatusOK, status)
}

func TestNewOperationalServer_DoesNotServeThePrimarysRoutes(t *testing.T) {
	t.Parallel()
	primary := newPrimary()
	opsAddr := serveOn(t, NewOperationalServer(primary, OperationalConfig{}, WithLogger(zerolog.Nop())))

	status, _ := getPath(t, opsAddr, "/api/things")

	assert.Equal(t, http.StatusNotFound, status)
}

func TestNewOperationalServer_ReportsResourcesAddedToThePrimaryLater(t *testing.T) {
	t.Parallel()
	primary := newPrimary()
	ops := NewOperationalServer(primary, OperationalConfig{}, WithLogger(zerolog.Nop()))
	primary.AddResource("/store", readyResource{err: errors.New("store unreachable")})
	opsAddr := serveOn(t, ops)

	status, body := getPath(t, opsAddr, "/health/ready")

	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.Contains(t, body, "store unreachable")
}

func TestNewOperationalServer_ServesThePrimarysRequestMetrics(t *testing.T) {
	t.Parallel()
	primary := newPrimary()
	ops := NewOperationalServer(primary, OperationalConfig{EnablePrometheusMetrics: true}, WithLogger(zerolog.Nop()))
	primaryAddr, opsAddr := serveOn(t, primary), serveOn(t, ops)

	// The same path on both listeners: the operational server's own request
	// series must not collide with the primary's.
	getPath(t, primaryAddr, "/health/ready")
	getPath(t, opsAddr, "/health/ready")

	status, body := getPath(t, opsAddr, "/metrics")

	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, "dioad_net_http_requests_total")
	assert.Contains(t, body, `code="401"`, "the primary's rejected request is counted")
}

func TestNewOperationalServer_StatusAndDebugAreOptIn(t *testing.T) {
	t.Parallel()
	opsAddr := serveOn(t, NewOperationalServer(newPrimary(), OperationalConfig{}, WithLogger(zerolog.Nop())))

	for _, path := range []string{"/metrics", "/status", "/debug/pprof/"} {
		status, _ := getPath(t, opsAddr, path)
		assert.Equal(t, http.StatusNotFound, status, path)
	}
}

func TestDedupGatherer_KeepsTheFirstCopyOfASeries(t *testing.T) {
	t.Parallel()
	newRegistry := func(value float64) *prometheus.Registry {
		reg := prometheus.NewRegistry()
		c := prometheus.NewCounter(prometheus.CounterOpts{Name: "things_total", Help: "Things."})
		c.Add(value)
		reg.MustRegister(c)
		return reg
	}

	families, err := dedupGatherer{newRegistry(1), newRegistry(2)}.Gather()

	require.NoError(t, err, "plain prometheus.Gatherers fails on the duplicate")
	require.Len(t, families, 1)
	require.Len(t, families[0].GetMetric(), 1)
	assert.InDelta(t, 1, families[0].GetMetric()[0].GetCounter().GetValue(), 0)

	_, err = prometheus.Gatherers{newRegistry(1), newRegistry(2)}.Gather()
	require.Error(t, err)
}
