package ratelimit

import (
	"sync"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/time/rate"
)

// limiterEntry tracks a rate limiter, when it was last used, and the outcome of the last allow check.
type limiterEntry struct {
	limiter  *rate.Limiter
	lastUsed time.Time
	// lastAllow records whether the most recent request for this entry was allowed.
	// This field is intentionally retained for observability and potential future logic
	// (e.g., metrics, debugging, or external consumers) even if not currently read here.
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
	lastCleanup       time.Time
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
		lastCleanup:       time.Now(),
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
		lastCleanup:     time.Now(),
		StaleTTL:        30 * time.Minute,
	}
}

// Allow checks if a request from the given principal is allowed.
func (rl *RateLimiter) Allow(principal string) bool {
	// Get rate limits (potentially from external source) before acquiring any locks
	rps := rl.RequestsPerSecond
	burst := rl.Burst

	if rl.LimitSource != nil {
		if sRps, sBurst, ok := rl.LimitSource.GetLimit(principal); ok {
			rps = sRps
			burst = sBurst
		}
	}

	// Try to get existing entry with read lock first
	rl.mu.RLock()
	entry, exists := rl.limiters[principal]
	rl.mu.RUnlock()

	// If entry doesn't exist, acquire write lock to create it
	if !exists {
		rl.mu.Lock()
		// Double-check that another goroutine didn't create it while we were waiting
		entry, exists = rl.limiters[principal]
		if !exists {
			entry = &limiterEntry{
				limiter:  rate.NewLimiter(rate.Limit(rps), burst),
				lastUsed: time.Now(),
			}
			rl.limiters[principal] = entry
		}
		rl.mu.Unlock()
	}

	// Update limits if they have changed (rate.Limiter methods are thread-safe)
	if entry.limiter.Limit() != rate.Limit(rps) {
		entry.limiter.SetLimit(rate.Limit(rps))
	}
	if entry.limiter.Burst() != burst {
		entry.limiter.SetBurst(burst)
	}

	// Check if allowed (rate.Limiter.Allow is thread-safe)
	allowed := entry.limiter.Allow()

	// Update entry metadata with a brief write lock
	// Re-verify the entry still exists and is the same entry
	rl.mu.Lock()
	if currentEntry, stillExists := rl.limiters[principal]; stillExists && currentEntry == entry {
		entry.lastUsed = time.Now()
		entry.lastAllow = allowed
	}
	// Also check if cleanup is needed while we have the lock
	needsCleanup := time.Since(rl.lastCleanup) > rl.CleanupInterval
	if needsCleanup {
		rl.cleanupExpiredLimiters()
	}
	rl.mu.Unlock()

	// Log rate limit exceeded outside of any locks
	if !allowed {
		rl.logger.Warn().
			Str("principal", principal).
			Float64("rps", rps).
			Int("burst", burst).
			Msg("rate limit exceeded for principal")
	}

	// Cleanup old limiters periodically
	if time.Since(rl.lastCleanup) > rl.CleanupInterval {
		rl.cleanupExpiredLimiters()
	}

	return allowed
}

// RetryAfter returns the duration until the next request would be allowed for the given principal.
// This can be used to set the Retry-After header in HTTP responses.
// If the principal has no limiter entry (first request), it returns 0.
func (rl *RateLimiter) RetryAfter(principal string) time.Duration {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	entry, exists := rl.limiters[principal]
	if !exists {
		return 0
	}

	// Reserve a token to check when the next one would be available
	r := entry.limiter.Reserve()
	delay := r.Delay()
	// Cancel the reservation so we don't actually consume a token
	r.Cancel()

	return delay
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

	rl.lastCleanup = now
}
