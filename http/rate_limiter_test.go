package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_Middleware(t *testing.T) {
	logger := zerolog.New(zerolog.NewConsoleWriter())
	// 1 token per second, burst of 1
	rl := NewRateLimiter(1, 1, logger)

	handler := rl.Middleware("user1")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestRateLimiter_MiddlewareFromContext(t *testing.T) {
	logger := zerolog.New(zerolog.NewConsoleWriter())
	rl := NewRateLimiter(1, 1, logger)

	type contextKey string
	key := contextKey("principal")

	handler := rl.MiddlewareFromContext(key)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request with principal in context
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), key, "user1"))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// Request without principal in context
	req = httptest.NewRequest("GET", "/", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	// Request with empty principal
	req = httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), key, ""))
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	// Request with principal that gets rate limited
	rl2 := NewRateLimiter(1, 1, logger)
	handler2 := rl2.MiddlewareFromContext(key)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req = httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), key, "user1"))
	rr = httptest.NewRecorder()
	handler2.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	handler2.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Equal(t, "1", rr.Header().Get("Retry-After"), "Retry-After should be 1 second for 1 req/sec limiter")
}

func TestRateLimiter_RetryAfterHeaderAccuracy(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name              string
		requestsPerSecond float64
		burst             int
		expectedRetryAfter string
	}{
		{
			name:              "1 request per second",
			requestsPerSecond: 1,
			burst:             1,
			expectedRetryAfter: "1",
		},
		{
			name:              "10 requests per second",
			requestsPerSecond: 10,
			burst:             1,
			expectedRetryAfter: "1", // ceil(0.1) = 1
		},
		{
			name:              "0.5 requests per second (1 per 2 seconds)",
			requestsPerSecond: 0.5,
			burst:             1,
			expectedRetryAfter: "2",
		},
		{
			name:              "100 requests per second",
			requestsPerSecond: 100,
			burst:             1,
			expectedRetryAfter: "1", // ceil(0.01) = 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(tt.requestsPerSecond, tt.burst, logger)

			handler := rl.Middleware("user1")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

