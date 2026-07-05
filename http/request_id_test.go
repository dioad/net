package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	diohttp "github.com/dioad/net/http"
)

func TestRequestIDMiddleware_setsResponseHeader(t *testing.T) {
	t.Parallel()

	handler := diohttp.RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.NotEmpty(t, rec.Header().Get("X-Request-ID"), "response must carry X-Request-ID")
}

func TestRequestIDMiddleware_ignoresClientSuppliedHeader(t *testing.T) {
	t.Parallel()

	const clientID = "client-supplied-id"

	handler := diohttp.RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", clientID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	id := rec.Header().Get("X-Request-ID")
	require.NotEmpty(t, id)
	assert.NotEqual(t, clientID, id, "server must generate a fresh ID, not echo the client's")
}

func TestRequestIDMiddleware_injectsRequestIDIntoContextLogger(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	handler := diohttp.RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		zerolog.Ctx(r.Context()).Info().Msg("test")
		w.WriteHeader(http.StatusOK)
	}))

	logger := zerolog.New(&buf)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(logger.WithContext(req.Context()))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	responseID := rec.Header().Get("X-Request-ID")
	require.NotEmpty(t, responseID)

	var logEntry map[string]string
	require.NoError(t, json.Unmarshal(buf.Bytes(), &logEntry))
	assert.Equal(t, responseID, logEntry["request_id"],
		"context logger must carry the same request_id that was set on the response")
}

func TestRequestIDMiddleware_worksWithoutPreseededLogger(t *testing.T) {
	t.Parallel()

	handler := diohttp.RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should not panic even when no logger was in the context before.
		_ = zerolog.Ctx(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.NotEmpty(t, rec.Header().Get("X-Request-ID"))
}

// TestRequestIDMiddleware_requestIDVisibleFromOuterContext verifies that a
// logger pointer captured BEFORE RequestIDMiddleware runs (as hlog.AccessHandler
// does) still sees request_id after the middleware completes.
// This is the access-log correlation scenario: the middleware must mutate the
// logger in-place rather than forking to a new context.
func TestRequestIDMiddleware_requestIDVisibleFromOuterContext(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(logger.WithContext(req.Context()))

	// Capture the logger pointer before the middleware runs, exactly as
	// hlog.AccessHandler captures it when it wraps the next handler.
	outerLogger := zerolog.Ctx(req.Context())

	handler := diohttp.RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	responseID := rec.Header().Get("X-Request-ID")
	require.NotEmpty(t, responseID)

	// Emit a deferred log event via the pre-captured pointer, simulating the
	// access log writing after ServeHTTP returns.
	outerLogger.Info().Msg("access log")

	var logEntry map[string]string
	require.NoError(t, json.Unmarshal(buf.Bytes(), &logEntry))
	assert.Equal(t, responseID, logEntry["request_id"],
		"pre-middleware logger pointer must reflect request_id added by UpdateContext")
}

func TestRequestIDMiddleware_generatesDistinctIDsPerRequest(t *testing.T) {
	t.Parallel()

	handler := diohttp.RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ids := make(map[string]struct{}, 10)
	for range 10 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		id := rec.Header().Get("X-Request-ID")
		require.NotEmpty(t, id)
		_, exists := ids[id]
		assert.False(t, exists, "duplicate request ID generated: %s", id)
		ids[id] = struct{}{}
	}
}
