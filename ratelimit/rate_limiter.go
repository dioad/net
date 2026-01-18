package ratelimit

import (
	"sync"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/time/rate"
)

// limiterEntry tracks a rate limiter and when it was last used.
type limiterEntry struct {
	limiter   *rate.Limiter
	lastUsed  time.Time
	lastAllow bool
}

// RateLimitSource defines the interface for determining rate limits.
type RateLimitSource interface {
	// GetLimit returns the rate limits to apply for a principal.
	// If it returns ok=false, the default limits of the RateLimiter will be used.
	GetLimit(principal string) (requestsPerSecond float64, burst int, ok bool)
}

// StaticRateLimitSource is a simple implementation of RateLimitSource that returns fixed limits.
type StaticRateLimitSource struct {
	RequestsPerSecond float64
	Burst             int
}

// GetLimit returns the fixed limits.
func (s *StaticRateLimitSource) GetLimit(principal string) (float64, int, bool) {
	return s.RequestsPerSecond, s.Burst, true
}

// RateLimiter provides per-principal rate limiting.
// It tracks last-used time for each principal and cleans up stale limiters.
type RateLimiter struct {
	limiters map[string]*limiterEntry
	mu       sync.RWMutex
	logger   zerolog.Logger

	// Configuration
	RequestsPerSecond float64
	Burst             int
	CleanupInterval   time.Duration
	LastCleanup       time.Time
	StaleTTL          time.Duration

	// LimitSource provides dynamic rate limits per principal.
	LimitSource RateLimitSource
}

// NewRateLimiter creates a new rate limiter with static limits.
// requestsPerSecond: allowed requests per second per principal
// burst: maximum burst size
func NewRateLimiter(requestsPerSecond float64, burst int, logger zerolog.Logger) *RateLimiter {
	return &RateLimiter{
		limiters:          make(map[string]*limiterEntry),
		logger:            logger,
		RequestsPerSecond: requestsPerSecond,
		Burst:             burst,
		CleanupInterval:   5 * time.Minute, // Cleanup every 5 minutes
		LastCleanup:       time.Now(),
		StaleTTL:          30 * time.Minute, // Remove limiters unused for 30 minutes
	}
}

// NewRateLimiterWithSource creates a new rate limiter with a custom rate limit source.
func NewRateLimiterWithSource(source RateLimitSource, logger zerolog.Logger) *RateLimiter {
	return &RateLimiter{
		limiters:        make(map[string]*limiterEntry),
		logger:          logger,
		LimitSource:     source,
		CleanupInterval: 5 * time.Minute,
		LastCleanup:     time.Now(),
		StaleTTL:        30 * time.Minute,
	}
}

// Allow checks if a request from the given principal is allowed.
func (rl *RateLimiter) Allow(principal string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rps := rl.RequestsPerSecond
	burst := rl.Burst

	if rl.LimitSource != nil {
		if sRps, sBurst, ok := rl.LimitSource.GetLimit(principal); ok {
			rps = sRps
			burst = sBurst
		}
	}

	entry, exists := rl.limiters[principal]
	if !exists {
		entry = &limiterEntry{
			limiter:  rate.NewLimiter(rate.Limit(rps), burst),
			lastUsed: time.Now(),
		}
		rl.limiters[principal] = entry
	} else {
		// Update limits if they have changed
		if entry.limiter.Limit() != rate.Limit(rps) {
			entry.limiter.SetLimit(rate.Limit(rps))
		}
		if entry.limiter.Burst() != burst {
			entry.limiter.SetBurst(burst)
		}
	}

	entry.lastUsed = time.Now()
	allowed := entry.limiter.Allow()
	entry.lastAllow = allowed

	if !allowed {
		rl.logger.Warn().
			Str("principal", principal).
			Float64("rps", rps).
			Int("burst", burst).
			Msg("rate limit exceeded for principal")
	}

	// Cleanup old limiters periodically
	if time.Since(rl.LastCleanup) > rl.CleanupInterval {
		rl.cleanupExpiredLimiters()
	}

	return allowed
}

// cleanupExpiredLimiters removes limiters that haven't been used recently.
// This prevents unbounded memory growth from unique principals.
func (rl *RateLimiter) cleanupExpiredLimiters() {
	staleCount := 0
	now := time.Now()

	for principal, entry := range rl.limiters {
		if now.Sub(entry.lastUsed) > rl.StaleTTL {
			delete(rl.limiters, principal)
			staleCount++
		}
	}

	if staleCount > 0 {
		rl.logger.Info().
			Int("removed_limiters", staleCount).
			Int("remaining_limiters", len(rl.limiters)).
			Msg("cleaned up stale rate limiters")
	}

	rl.LastCleanup = now
}
