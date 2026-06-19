package http

import (
	"errors"
	"fmt"
	"math"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
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
	counter           *prometheus.CounterVec
	registry          prometheus.Registerer
}

// WithPrincipalFunc allows configuring the function used to extract the principal from incoming HTTP requests.
func WithPrincipalFunc(getPrincipal PrincipalFunc) func(*RateLimiter) {
	return func(rl *RateLimiter) {
		rl.getPrincipal = getPrincipal
	}
}

// WithRateLimitSource allows configuring a dynamic rate limit source that can provide rate limits based on the principal or other factors.
// Note: WithRateLimitSource and WithStaticRateLimit are mutually exclusive. If both are configured, the source takes precedence.
func WithRateLimitSource(source ratelimit.RateLimitSource) func(*RateLimiter) {
	return func(rl *RateLimiter) {
		rl.source = source
	}
}

// WithStaticRateLimit allows configuring static rate limits with a specified number of requests per second and burst size.
// Note: WithStaticRateLimit and WithRateLimitSource are mutually exclusive. If both are configured, static limits are ignored.
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

// WithRateLimiterRegistry sets the Prometheus registry used to register the rate-limit
// counter. When not set, prometheus.DefaultRegisterer is used.
func WithRateLimiterRegistry(reg prometheus.Registerer) func(*RateLimiter) {
	return func(rl *RateLimiter) {
		rl.registry = reg
	}
}

// RateLimiterOption is a functional option for configuring an HTTP RateLimiter.
type RateLimiterOption func(*RateLimiter)

// ClientIPPrincipalFunc is a default PrincipalFunc that extracts the client's IP address from the request for rate limiting purposes.
func ClientIPPrincipalFunc(r *http.Request) (string, error) {
	return GetClientIP(r), nil
}

// StaticPrincipalFunc returns a PrincipalFunc that always returns the given principal.
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
		requestsPerSecond: DefaultRequestsPerSecond,
		burst:             DefaultBurst,
		logger:            zerolog.Nop(),
	}

	for _, opt := range opts {
		opt(r)
	}

	rlOpts := []ratelimit.Option{
		ratelimit.WithRateLimiterLogger(r.logger),
		ratelimit.WithRateLimiterStaticLimits(r.requestsPerSecond, r.burst),
	}
	if r.source != nil {
		rlOpts = append(rlOpts, ratelimit.WithRateLimiterSource(r.source))
	}
	r.limiter = ratelimit.NewRateLimiterWithOptions(rlOpts...)

	reg := r.registry
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	r.counter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dioad_net_http_rate_limit_requests_total",
			Help: "Count of requests evaluated by rate limiter.",
		},
		[]string{"result"},
	)
	if err := reg.Register(r.counter); err != nil {
		var are prometheus.AlreadyRegisteredError
		if errors.As(err, &are) {
			if existing, ok := are.ExistingCollector.(*prometheus.CounterVec); ok {
				r.counter = existing
			} else {
				r.logger.Error().Err(err).Msg("rate-limit counter registration conflict: existing collector has unexpected type")
			}
		} else {
			r.logger.Error().Err(err).Msg("rate-limit counter registration failed; metrics will not be exported")
		}
	}

	return r
}

// Stop shuts down the background cleanup goroutine started by the rate limiter.
// It should be called when the RateLimiter is no longer needed to avoid a goroutine leak.
// Stop is safe to call multiple times.
func (rl *RateLimiter) Stop() {
	rl.limiter.Stop()
}

// setRetryAfterHeader calculates and sets the Retry-After header based on the rate limiter state.
func (rl *RateLimiter) setRetryAfterHeader(w http.ResponseWriter, principal string) {
	retryAfter := rl.limiter.RetryAfter(principal)
	retryAfterSeconds := max(
		int(math.Ceil(retryAfter.Seconds())), 1)
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
			rl.counter.WithLabelValues("blocked").Inc()
			rl.setRetryAfterHeader(w, p)
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		rl.counter.WithLabelValues("allowed").Inc()
		next.ServeHTTP(w, r)
	})
}
