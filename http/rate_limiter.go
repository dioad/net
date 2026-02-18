package http

import (
	"fmt"
	"math"
	"net/http"

	"github.com/rs/zerolog"

	"github.com/dioad/net/ratelimit"
)

var (
	DefaultRequestsPerSecond = float64(10) // default to 10 rps
	DefaultBurst             = 20          // DefaultBurst specifies the default maximum burst size for rate limiting.
)

// PrincipalFunc defines a function type that extracts a principal identifier from an HTTP request for rate limiting purposes.
type PrincipalFunc func(*http.Request) (string, error)

// RateLimiter provides per-principal rate limiting for HTTP requests.
type RateLimiter struct {
	limiter           *ratelimit.RateLimiter
	getPrincipal      PrincipalFunc
	source            ratelimit.RateLimitSource
	requestsPerSecond float64
	burst             int
	logger            zerolog.Logger
}

// WithPrincipalFunc allows configuring the function used to extract the principal from incoming HTTP requests.
func WithPrincipalFunc(getPrincipal PrincipalFunc) func(*RateLimiter) {
	return func(rl *RateLimiter) {
		rl.getPrincipal = getPrincipal
	}
}

// WithRateLimitSource allows configuring a dynamic rate limit source that can provide rate limits based on the principal or other factors.
func WithRateLimitSource(source ratelimit.RateLimitSource) func(*RateLimiter) {
	return func(rl *RateLimiter) {
		rl.source = source
	}
}

// WithStaticRateLimit allows configuring static rate limits with a specified number of requests per second and burst size.
func WithStaticRateLimit(requestsPerSecond float64, burst int) func(*RateLimiter) {
	return func(rl *RateLimiter) {
		rl.requestsPerSecond = requestsPerSecond
		rl.burst = burst
	}
}

// WithRateLimitLogger allows configuring a logger for the rate limiter to log rate limit events and decisions.
func WithRateLimitLogger(logger zerolog.Logger) func(*RateLimiter) {
	return func(rl *RateLimiter) {
		rl.logger = logger
	}
}

type RateLimiterOption func(*RateLimiter)

// ClientIPPrincipalFunc is a default PrincipalFunc that extracts the client's IP address from the request for rate limiting purposes.
func ClientIPPrincipalFunc(r *http.Request) (string, error) {
	return GetClientIP(r), nil
}

func StaticPrincipalFunc(principal string) PrincipalFunc {
	return func(r *http.Request) (string, error) {
		return principal, nil
	}
}

// NewRateLimiter creates a new rate limiter with static limits.
// requestsPerSecond: allowed requests per second per principal
// burst: maximum burst size
func NewRateLimiter(opts ...RateLimiterOption) *RateLimiter {
	r := &RateLimiter{
		getPrincipal:      ClientIPPrincipalFunc,
		requestsPerSecond: DefaultRequestsPerSecond, // default to 10 rps
		burst:             DefaultBurst,             // default burst size
		logger:            zerolog.Nop(),            // default to no-op logger
	}

	for _, opt := range opts {
		opt(r)
	}

	rateLimiter := ratelimit.NewRateLimiter(r.requestsPerSecond, r.burst, r.logger)
	if r.source != nil {
		rateLimiter = ratelimit.NewRateLimiterWithSource(r.source, r.logger)
	}

	r.limiter = rateLimiter

	return r
}

// setRetryAfterHeader calculates and sets the Retry-After header based on the rate limiter state.
func (rl *RateLimiter) setRetryAfterHeader(w http.ResponseWriter, principal string) {
	retryAfter := rl.limiter.RetryAfter(principal)
	retryAfterSeconds := int(math.Ceil(retryAfter.Seconds()))
	// Ensure a minimum of 1 second for Retry-After
	if retryAfterSeconds < 1 {
		retryAfterSeconds = 1
	}
	w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfterSeconds))
}

// Middleware returns an HTTP middleware for rate limiting.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := rl.getPrincipal(r)
		if err != nil {
			http.Error(w, "unable to determine principal for rate limiting", http.StatusBadRequest)
			return
		}
		if !rl.limiter.Allow(p) {
			rateLimitRequests.WithLabelValues("blocked").Inc()
			rl.setRetryAfterHeader(w, p)
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		rateLimitRequests.WithLabelValues("allowed").Inc()
		next.ServeHTTP(w, r)
	})

}
