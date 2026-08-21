package http

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter_Middleware(t *testing.T) {
	logger := zerolog.New(zerolog.NewConsoleWriter())
	// 1 token per second, burst of 1
	rl := NewRateLimiter(
		WithStaticRateLimit(1, 1),
		WithRateLimitLogger(logger),
		WithPrincipalFunc(StaticPrincipalFunc("user1")),
	)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request - allowed
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// Second request - rate limited
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Equal(t, "1", rr.Header().Get("Retry-After"), "Retry-After should be 1 second for 1 req/sec limiter")
}

func TestRateLimiter_Middleware_LogsRejectionViaRequestScopedLogger(t *testing.T) {
	// The RateLimiter itself is constructed with a Nop logger to prove the
	// rejection warning does NOT come from the construction-time logger -
	// it must come from whatever logger is embedded in the request context.
	rl := NewRateLimiter(
		WithStaticRateLimit(1, 1),
		WithRateLimitLogger(zerolog.Nop()),
		WithPrincipalFunc(StaticPrincipalFunc("user1")),
	)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	var buf bytes.Buffer
	requestLogger := zerolog.New(&buf)

	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(requestLogger.WithContext(req.Context()))

	// First request - allowed, no log expected.
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Empty(t, buf.String(), "no warning should be logged for an allowed request")

	// Second request - rejected, must log via the request-scoped logger.
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusTooManyRequests, rr.Code)

	logged := buf.String()
	assert.Contains(t, logged, "rate limit exceeded for principal")
	assert.Contains(t, logged, `"principal":"user1"`)
}

func TestRateLimiter_RetryAfterHeaderAccuracy(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name               string
		requestsPerSecond  float64
		burst              int
		expectedRetryAfter string
	}{
		{
			name:               "1 request per second",
			requestsPerSecond:  1,
			burst:              1,
			expectedRetryAfter: "1",
		},
		{
			name:               "10 requests per second",
			requestsPerSecond:  10,
			burst:              1,
			expectedRetryAfter: "1", // ceil(0.1) = 1
		},
		{
			name:               "0.5 requests per second (1 per 2 seconds)",
			requestsPerSecond:  0.5,
			burst:              1,
			expectedRetryAfter: "2",
		},
		{
			name:               "100 requests per second",
			requestsPerSecond:  100,
			burst:              1,
			expectedRetryAfter: "1", // ceil(0.01) = 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(
				WithStaticRateLimit(tt.requestsPerSecond, tt.burst),
				WithRateLimitLogger(logger),
				WithPrincipalFunc(StaticPrincipalFunc("user1")))

			handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			// First request - allowed (uses up burst)
			req := httptest.NewRequest("GET", "/", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)

			// Second request - rate limited
			rr = httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusTooManyRequests, rr.Code)
			assert.Equal(t, tt.expectedRetryAfter, rr.Header().Get("Retry-After"),
				"Retry-After header should match the calculated value based on rate limit")
		})
	}
}

func TestRateLimiter_DefaultClientIPPrincipalFunc(t *testing.T) {
	logger := zerolog.Nop()
	// 1 token per second, burst of 1
	rl := NewRateLimiter(
		WithStaticRateLimit(1, 1),
		WithRateLimitLogger(logger),
	)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req1 := httptest.NewRequest("GET", "/", nil)
	req1.Header.Set("X-Forwarded-For", "10.0.0.1")
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("X-Forwarded-For", "10.0.0.2")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req1)
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req2)
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req1)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestRateLimiter_PrincipalFuncError(t *testing.T) {
	logger := zerolog.Nop()
	rl := NewRateLimiter(
		WithStaticRateLimit(1, 1),
		WithRateLimitLogger(logger),
		WithPrincipalFunc(func(r *http.Request) (string, error) {
			return "", errors.New("missing principal")
		}),
	)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "unable to determine principal for rate limiting")
}

func TestRateLimiter_CustomPrincipalFuncFromContext(t *testing.T) {
	logger := zerolog.Nop()
	type principalKey struct{}

	rl := NewRateLimiter(
		WithStaticRateLimit(1, 1),
		WithRateLimitLogger(logger),
		WithPrincipalFunc(func(r *http.Request) (string, error) {
			principal, ok := r.Context().Value(principalKey{}).(string)
			if !ok {
				return "", errors.New("missing principal")
			}
			return principal, nil
		}),
	)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), principalKey{}, "user1"))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestRateLimiter_EmptyPrincipal(t *testing.T) {
	logger := zerolog.Nop()
	rl := NewRateLimiter(
		WithStaticRateLimit(1, 1),
		WithRateLimitLogger(logger),
		WithPrincipalFunc(func(r *http.Request) (string, error) {
			return "", nil
		}),
	)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}
