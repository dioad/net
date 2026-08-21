package ratelimit

import (
	"context"
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

	// Configuration (read-only after creation to avoid data races)
	requestsPerSecond float64
	burst             int
	cleanupInterval   time.Duration
	staleTTL          time.Duration

	// LimitSource provides dynamic rate limits per principal.
	LimitSource RateLimitSource

	// Background cleanup
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	stopOnce sync.Once
}

// NewRateLimiter creates a new rate limiter with static limits.
//
// Deprecated: Use NewRateLimiterWithOptions with WithRateLimiterStaticLimits and
// WithRateLimiterLogger instead.
func NewRateLimiter(requestsPerSecond float64, burst int, logger zerolog.Logger) *RateLimiter {
	return NewRateLimiterWithConfig(requestsPerSecond, burst, 5*time.Minute, 30*time.Minute, logger)
}

// NewRateLimiterWithContext creates a new rate limiter with static limits and a context.
//
// Deprecated: Use NewRateLimiterWithOptions with WithRateLimiterContext,
// WithRateLimiterStaticLimits, and WithRateLimiterLogger instead.
func NewRateLimiterWithContext(ctx context.Context, requestsPerSecond float64, burst int, logger zerolog.Logger) *RateLimiter {
	return NewRateLimiterWithContextAndConfig(ctx, requestsPerSecond, burst, 5*time.Minute, 30*time.Minute, logger)
}

// NewRateLimiterWithConfig creates a new rate limiter with custom configuration.
//
// Deprecated: Use NewRateLimiterWithOptions with WithRateLimiterStaticLimits,
// WithRateLimiterCleanupConfig, and WithRateLimiterLogger instead.
func NewRateLimiterWithConfig(requestsPerSecond float64, burst int, cleanupInterval, staleTTL time.Duration, logger zerolog.Logger) *RateLimiter {
	if requestsPerSecond < 0 {
		requestsPerSecond = 0
	}
	if burst < 0 {
		burst = 0
	}
	if cleanupInterval <= 0 {
		cleanupInterval = 5 * time.Minute
	}
	if staleTTL <= 0 {
		staleTTL = 30 * time.Minute
	}

	ctx, cancel := context.WithCancel(context.Background()) // #nosec G118 -- cancel is stored on RateLimiter.cancel and invoked by Stop()
	rl := &RateLimiter{
		limiters:          make(map[string]*limiterEntry),
		logger:            logger,
		requestsPerSecond: requestsPerSecond,
		burst:             burst,
		cleanupInterval:   cleanupInterval,
		staleTTL:          staleTTL,
		ctx:               ctx,
		cancel:            cancel,
	}
	rl.start()
	return rl
}

// NewRateLimiterWithContextAndConfig creates a new rate limiter with custom configuration and a context.
//
// Deprecated: Use NewRateLimiterWithOptions with WithRateLimiterContext,
// WithRateLimiterStaticLimits, WithRateLimiterCleanupConfig, and WithRateLimiterLogger instead.
func NewRateLimiterWithContextAndConfig(ctx context.Context, requestsPerSecond float64, burst int, cleanupInterval, staleTTL time.Duration, logger zerolog.Logger) *RateLimiter {
	if requestsPerSecond < 0 {
		requestsPerSecond = 0
	}
	if burst < 0 {
		burst = 0
	}
	if cleanupInterval <= 0 {
		cleanupInterval = 5 * time.Minute
	}
	if staleTTL <= 0 {
		staleTTL = 30 * time.Minute
	}

	derivedCtx, cancel := context.WithCancel(ctx) // #nosec G118 -- cancel is stored on RateLimiter.cancel and invoked by Stop()
	rl := &RateLimiter{
		limiters:          make(map[string]*limiterEntry),
		logger:            logger,
		requestsPerSecond: requestsPerSecond,
		burst:             burst,
		cleanupInterval:   cleanupInterval,
		staleTTL:          staleTTL,
		ctx:               derivedCtx,
		cancel:            cancel,
	}
	rl.start()
	return rl
}

// NewRateLimiterWithSource creates a new rate limiter with a custom rate limit source.
//
// Deprecated: Use NewRateLimiterWithOptions with WithRateLimiterSource and
// WithRateLimiterLogger instead.
func NewRateLimiterWithSource(source RateLimitSource, logger zerolog.Logger) *RateLimiter {
	return NewRateLimiterWithSourceAndConfig(source, 5*time.Minute, 30*time.Minute, logger)
}

// NewRateLimiterWithSourceAndContext creates a new rate limiter with a custom rate limit source and a context.
//
// Deprecated: Use NewRateLimiterWithOptions with WithRateLimiterContext,
// WithRateLimiterSource, and WithRateLimiterLogger instead.
func NewRateLimiterWithSourceAndContext(ctx context.Context, source RateLimitSource, logger zerolog.Logger) *RateLimiter {
	return NewRateLimiterWithSourceContextAndConfig(ctx, source, 5*time.Minute, 30*time.Minute, logger)
}

// NewRateLimiterWithSourceAndConfig creates a new rate limiter with a custom rate limit source and configuration.
//
// Deprecated: Use NewRateLimiterWithOptions with WithRateLimiterSource,
// WithRateLimiterCleanupConfig, and WithRateLimiterLogger instead.
func NewRateLimiterWithSourceAndConfig(source RateLimitSource, cleanupInterval, staleTTL time.Duration, logger zerolog.Logger) *RateLimiter {
	if cleanupInterval <= 0 {
		cleanupInterval = 5 * time.Minute
	}
	if staleTTL <= 0 {
		staleTTL = 30 * time.Minute
	}

	ctx, cancel := context.WithCancel(context.Background()) // #nosec G118 -- cancel is stored on RateLimiter.cancel and invoked by Stop()
	rl := &RateLimiter{
		limiters:        make(map[string]*limiterEntry),
		logger:          logger,
		LimitSource:     source,
		cleanupInterval: cleanupInterval,
		staleTTL:        staleTTL,
		ctx:             ctx,
		cancel:          cancel,
	}
	rl.start()
	return rl
}

// NewRateLimiterWithSourceContextAndConfig creates a new rate limiter with a custom rate limit source,
// context, and configuration.
//
// Deprecated: Use NewRateLimiterWithOptions with WithRateLimiterContext,
// WithRateLimiterSource, WithRateLimiterCleanupConfig, and WithRateLimiterLogger instead.
func NewRateLimiterWithSourceContextAndConfig(ctx context.Context, source RateLimitSource, cleanupInterval, staleTTL time.Duration, logger zerolog.Logger) *RateLimiter {
	if cleanupInterval <= 0 {
		cleanupInterval = 5 * time.Minute
	}
	if staleTTL <= 0 {
		staleTTL = 30 * time.Minute
	}

	derivedCtx, cancel := context.WithCancel(ctx) // #nosec G118 -- cancel is stored on RateLimiter.cancel and invoked by Stop()
	rl := &RateLimiter{
		limiters:        make(map[string]*limiterEntry),
		logger:          logger,
		LimitSource:     source,
		cleanupInterval: cleanupInterval,
		staleTTL:        staleTTL,
		ctx:             derivedCtx,
		cancel:          cancel,
	}
	rl.start()
	return rl
}

// Option is a functional option for configuring a RateLimiter.
type Option func(*rateLimiterOptions)

type rateLimiterOptions struct {
	ctx             context.Context
	requestsPerSec  float64
	burst           int
	cleanupInterval time.Duration
	staleTTL        time.Duration
	limitSource     RateLimitSource
	logger          zerolog.Logger
}

// WithRateLimiterContext sets a parent context whose cancellation stops
// the background cleanup goroutine.
func WithRateLimiterContext(ctx context.Context) Option {
	return func(o *rateLimiterOptions) { o.ctx = ctx }
}

// WithRateLimiterStaticLimits sets the static requests-per-second and burst values.
func WithRateLimiterStaticLimits(rps float64, burst int) Option {
	return func(o *rateLimiterOptions) {
		o.requestsPerSec = rps
		o.burst = burst
	}
}

// WithRateLimiterSource sets a dynamic RateLimitSource. When set, per-principal
// limits from the source take precedence over the static limits.
func WithRateLimiterSource(s RateLimitSource) Option {
	return func(o *rateLimiterOptions) { o.limitSource = s }
}

// WithRateLimiterCleanupConfig sets the cleanup interval and stale-entry TTL.
func WithRateLimiterCleanupConfig(cleanupInterval, staleTTL time.Duration) Option {
	return func(o *rateLimiterOptions) {
		o.cleanupInterval = cleanupInterval
		o.staleTTL = staleTTL
	}
}

// WithRateLimiterLogger sets the logger used by the rate limiter.
func WithRateLimiterLogger(logger zerolog.Logger) Option {
	return func(o *rateLimiterOptions) { o.logger = logger }
}

// NewRateLimiterWithOptions creates a RateLimiter using functional options.
// This is the preferred constructor; the positional constructors are deprecated.
func NewRateLimiterWithOptions(opts ...Option) *RateLimiter {
	o := &rateLimiterOptions{
		ctx:             context.Background(),
		requestsPerSec:  0,
		burst:           0,
		cleanupInterval: 5 * time.Minute,
		staleTTL:        30 * time.Minute,
	}
	for _, opt := range opts {
		opt(o)
	}

	if o.cleanupInterval <= 0 {
		o.cleanupInterval = 5 * time.Minute
	}
	if o.staleTTL <= 0 {
		o.staleTTL = 30 * time.Minute
	}

	ctx, cancel := context.WithCancel(o.ctx) // #nosec G118 -- cancel is stored on RateLimiter.cancel and invoked by Stop()
	rl := &RateLimiter{
		limiters:          make(map[string]*limiterEntry),
		logger:            o.logger,
		requestsPerSecond: o.requestsPerSec,
		burst:             o.burst,
		cleanupInterval:   o.cleanupInterval,
		staleTTL:          o.staleTTL,
		LimitSource:       o.limitSource,
		ctx:               ctx,
		cancel:            cancel,
	}
	rl.start()
	return rl
}

// Allow checks if a request from the given principal is allowed.
//
// Allow does not log rejections itself; it is a pure decision. Callers with
// access to a request-scoped logger (e.g. an HTTP middleware) should log the
// rejection themselves using that logger, rather than relying on a logger
// captured at RateLimiter construction time.
func (rl *RateLimiter) Allow(principal string) bool {
	// Get rate limits (potentially from external source) before acquiring any locks
	rps := rl.requestsPerSecond
	burst := rl.burst

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
	rl.mu.Unlock()

	return allowed
}

// RetryAfter returns the duration until the next request would be allowed for the given principal.
// This can be used to set the Retry-After header in HTTP responses.
// If the principal has no limiter entry (first request), it returns 0.
// Note: This method uses RLock because it only reads from the limiters map. The Reserve/Cancel
// calls on the underlying rate.Limiter are thread-safe due to rate.Limiter's internal mutex.
// This method is typically called immediately after Allow() returns false, so the limiter entry
// will exist. If rate limits change between calls, the returned duration reflects the current
// limits at the time of the Reserve() call, which is acceptable for advisory Retry-After headers.
func (rl *RateLimiter) RetryAfter(principal string) time.Duration {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	entry, exists := rl.limiters[principal]
	if !exists {
		return 0
	}

	// Reserve a token to check when the next one would be available.
	// The rate.Limiter.Reserve() and Cancel() methods are thread-safe.
	r := entry.limiter.Reserve()
	delay := r.Delay()
	// Cancel the reservation so we don't actually consume a token
	r.Cancel()

	return delay
}

// start begins the background cleanup goroutine.
func (rl *RateLimiter) start() {
	rl.wg.Add(1)
	go rl.cleanupLoop()
}

// Stop gracefully stops the background cleanup goroutine.
// It should be called when the RateLimiter is no longer needed.
// Stop can be safely called multiple times.
// Note: When using context-based constructors (NewRateLimiterWithContext, etc.),
// calling Stop is not necessary as cleanup happens automatically when the context is cancelled.
// However, calling Stop after context cancellation is safe and will wait for cleanup to complete.
func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() {
		rl.cancel()
	})
	rl.wg.Wait()
}

// cleanupLoop runs in the background and periodically cleans up expired limiters.
func (rl *RateLimiter) cleanupLoop() {
	defer rl.wg.Done()

	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.ctx.Done():
			return
		case <-ticker.C:
			rl.cleanupExpiredLimiters()
		}
	}
}

// cleanupExpiredLimiters removes limiters that haven't been used recently.
// This prevents unbounded memory growth from unique principals.
func (rl *RateLimiter) cleanupExpiredLimiters() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	staleCount := 0
	now := time.Now()

	for principal, entry := range rl.limiters {
		if now.Sub(entry.lastUsed) > rl.staleTTL {
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
}
