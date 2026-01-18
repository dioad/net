package http

import (
	"net/http"

	"github.com/dioad/net/ratelimit"
	"github.com/rs/zerolog"
)

// RateLimiter provides per-principal rate limiting for HTTP requests.
type RateLimiter struct {
	*ratelimit.RateLimiter
}

// NewRateLimiter creates a new rate limiter with static limits.
// requestsPerSecond: allowed requests per second per principal
// burst: maximum burst size
func NewRateLimiter(requestsPerSecond float64, burst int, logger zerolog.Logger) *RateLimiter {
	return &RateLimiter{
		RateLimiter: ratelimit.NewRateLimiter(requestsPerSecond, burst, logger),
	}
}

// NewRateLimiterWithSource creates a new rate limiter with a custom rate limit source.
func NewRateLimiterWithSource(source ratelimit.RateLimitSource, logger zerolog.Logger) *RateLimiter {
	return &RateLimiter{
		RateLimiter: ratelimit.NewRateLimiterWithSource(source, logger),
	}
}

// Middleware returns an HTTP middleware for rate limiting.
func (rl *RateLimiter) Middleware(principal string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.Allow(principal) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MiddlewareFromContext returns an HTTP middleware that extracts the principal from context.
func (rl *RateLimiter) MiddlewareFromContext(contextKey interface{}) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := r.Context().Value(contextKey).(string)
			if !ok || principal == "" {
				http.Error(w, "unable to determine principal for rate limiting", http.StatusBadRequest)
				return
			}

			if !rl.Allow(principal) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
