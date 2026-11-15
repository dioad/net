package breaker

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"github.com/rs/zerolog"
)

// Defaults for RetryConfig, used when a field is zero.
const (
	DefaultMaxAttempts    = 3
	DefaultInitialBackoff = 100 * time.Millisecond
	DefaultMaxBackoff     = 30 * time.Second
	DefaultMultiplier     = 2.0
)

// RetryConfig tunes Retry. Zero values select the defaults.
type RetryConfig struct {
	// MaxAttempts is the number of calls made in all, including the first.
	MaxAttempts int
	// InitialBackoff is the delay before the first retry, before jitter.
	InitialBackoff time.Duration
	// MaxBackoff caps the delay before any retry, before jitter.
	MaxBackoff time.Duration
	// Multiplier is the factor the delay grows by after each retry.
	Multiplier float64
	// NoJitter turns jitter off. By default each delay is drawn uniformly
	// from zero to the backoff ("full jitter"), so clients that failed
	// together do not retry together.
	NoJitter bool
	// Retryable decides whether an error is worth another attempt. The
	// default retries every error except the caller's own cancellation and
	// ErrOpen, which a retry within the cool-down cannot fix.
	Retryable func(error) bool
	// Rand returns a value in [0, 1) for jitter; it defaults to a random
	// source and exists for tests.
	Rand func() float64
	// Sleep waits for d or until ctx is done; it defaults to a timer and
	// exists for tests.
	Sleep func(ctx context.Context, d time.Duration) error
}

func (c RetryConfig) withDefaults() RetryConfig {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = DefaultMaxAttempts
	}
	if c.InitialBackoff <= 0 {
		c.InitialBackoff = DefaultInitialBackoff
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = DefaultMaxBackoff
	}
	if c.Multiplier < 1 {
		c.Multiplier = DefaultMultiplier
	}
	if c.Retryable == nil {
		c.Retryable = defaultRetryable
	}
	if c.Rand == nil {
		c.Rand = rand.Float64
	}
	if c.Sleep == nil {
		c.Sleep = sleep
	}

	return c
}

func defaultRetryable(err error) bool {
	return !errors.Is(err, context.Canceled) && !errors.Is(err, ErrOpen)
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// backoff is the delay before retry number retry (1 for the first retry),
// before jitter.
func (c RetryConfig) backoff(retry int) time.Duration {
	d := float64(c.InitialBackoff) * math.Pow(c.Multiplier, float64(retry-1))

	return time.Duration(min(d, float64(c.MaxBackoff)))
}

func (c RetryConfig) delay(retry int) time.Duration {
	d := c.backoff(retry)
	if c.NoJitter {
		return d
	}

	return time.Duration(float64(d) * c.Rand())
}

// Retry calls fn until it succeeds, returns an error cfg does not consider
// retryable, or cfg.MaxAttempts calls have been made, waiting an
// exponentially growing, capped and jittered delay between calls. fn receives
// ctx. Progress is logged at debug level through the logger carried by ctx
// (zerolog.Ctx), if any.
//
// It returns nil on success and a non-retryable error unchanged. When
// attempts run out the error wraps the last one. When ctx is done it stops at
// once and returns an error carrying both ctx.Err() and the last error.
//
// To stop retrying while a dependency is down, call the breaker inside fn
// and treat ErrOpen as not retryable (the default).
func Retry(ctx context.Context, cfg RetryConfig, fn func(ctx context.Context) error) error {
	cfg = cfg.withDefaults()
	logger := zerolog.Ctx(ctx)

	var lastErr error
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return errors.Join(err, lastErr)
		}

		err := fn(ctx)
		if err == nil {
			return nil
		}
		lastErr = err

		if !cfg.Retryable(err) {
			return err
		}
		if attempt == cfg.MaxAttempts {
			break
		}

		delay := cfg.delay(attempt)
		logger.Debug().Err(err).Int("attempt", attempt).Int("max_attempts", cfg.MaxAttempts).
			Dur("retry_in", delay).Msg("attempt failed, retrying")

		if err := cfg.Sleep(ctx, delay); err != nil {
			return errors.Join(err, lastErr)
		}
	}

	return fmt.Errorf("giving up after %d attempts: %w", cfg.MaxAttempts, lastErr)
}
