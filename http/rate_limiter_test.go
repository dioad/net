package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
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
