package breaker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordSleeps returns a Sleep that records the delays it was asked for and
// never waits.
func recordSleeps(delays *[]time.Duration) func(context.Context, time.Duration) error {
	return func(_ context.Context, d time.Duration) error {
		*delays = append(*delays, d)

		return nil
	}
}

func Test_RetryConfig_backoff(t *testing.T) {
	cfg := RetryConfig{InitialBackoff: 100 * time.Millisecond, MaxBackoff: time.Second, Multiplier: 2}.withDefaults()

	var got []time.Duration
	for retry := 1; retry <= 6; retry++ {
		got = append(got, cfg.backoff(retry))
	}

	assert.Equal(t, []time.Duration{
		100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond, 800 * time.Millisecond,
		time.Second, time.Second,
	}, got, "the backoff doubles and is capped")
}

func Test_RetryConfig_backoff_DoesNotOverflow(t *testing.T) {
	cfg := RetryConfig{MaxBackoff: time.Minute}.withDefaults()

	assert.Equal(t, time.Minute, cfg.backoff(10_000))
}

func Test_RetryConfig_delay_Jitter(t *testing.T) {
	t.Run("full jitter scales the backoff by the random value", func(t *testing.T) {
		for _, r := range []float64{0, 0.25, 0.999} {
			cfg := RetryConfig{InitialBackoff: time.Second, Rand: func() float64 { return r }}.withDefaults()

			assert.Equal(t, time.Duration(float64(time.Second)*r), cfg.delay(1))
		}
	})

	t.Run("stays within bounds with the default source", func(t *testing.T) {
		cfg := RetryConfig{InitialBackoff: time.Second, MaxBackoff: 4 * time.Second}.withDefaults()

		for range 1000 {
			d := cfg.delay(3)
			assert.GreaterOrEqual(t, d, time.Duration(0))
			assert.LessOrEqual(t, d, 4*time.Second)
		}
	})

	t.Run("NoJitter returns the backoff", func(t *testing.T) {
		cfg := RetryConfig{InitialBackoff: time.Second, NoJitter: true}.withDefaults()

		assert.Equal(t, time.Second, cfg.delay(1))
	})
}

func Test_Retry_Success(t *testing.T) {
	var delays []time.Duration
	calls := 0

	err := Retry(t.Context(), RetryConfig{
		MaxAttempts: 5, InitialBackoff: time.Second, NoJitter: true, Sleep: recordSleeps(&delays),
	}, func(context.Context) error {
		calls++
		if calls < 3 {
			return errDown
		}

		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 3, calls)
	assert.Equal(t, []time.Duration{time.Second, 2 * time.Second}, delays)
}

func Test_Retry_ExhaustsAttempts(t *testing.T) {
	var delays []time.Duration
	calls := 0

	err := Retry(t.Context(), RetryConfig{MaxAttempts: 3, NoJitter: true, Sleep: recordSleeps(&delays)},
		func(context.Context) error {
			calls++

			return errDown
		})

	require.ErrorIs(t, err, errDown)
	require.ErrorContains(t, err, "giving up after 3 attempts")
	assert.Equal(t, 3, calls)
	assert.Len(t, delays, 2, "no wait after the last attempt")
}

func Test_Retry_NotRetryable(t *testing.T) {
	permanent := errors.New("permanent")

	t.Run("custom classifier returns the error unchanged", func(t *testing.T) {
		calls := 0

		err := Retry(t.Context(), RetryConfig{
			Retryable: func(err error) bool { return !errors.Is(err, permanent) },
			Sleep:     recordSleeps(new([]time.Duration)),
		}, func(context.Context) error {
			calls++

			return permanent
		})

		assert.Equal(t, permanent, err)
		assert.Equal(t, 1, calls)
	})

	t.Run("ErrOpen is not retried by default", func(t *testing.T) {
		calls := 0

		err := Retry(t.Context(), RetryConfig{Sleep: recordSleeps(new([]time.Duration))}, func(context.Context) error {
			calls++

			return ErrOpen
		})

		require.ErrorIs(t, err, ErrOpen)
		assert.Equal(t, 1, calls)
	})
}

func Test_Retry_CancelledContext(t *testing.T) {
	t.Run("cancelled while waiting stops the loop", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		calls := 0

		err := Retry(ctx, RetryConfig{MaxAttempts: 10, InitialBackoff: time.Hour, NoJitter: true},
			func(context.Context) error {
				calls++
				cancel()

				return errDown
			})

		require.ErrorIs(t, err, context.Canceled)
		require.ErrorIs(t, err, errDown)
		assert.Equal(t, 1, calls)
	})

	t.Run("cancelled before the first attempt makes no call", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := Retry(ctx, RetryConfig{}, func(context.Context) error {
			t.Fatal("fn must not be called")

			return nil
		})

		require.ErrorIs(t, err, context.Canceled)
	})
}

func Test_Retry_Defaults(t *testing.T) {
	cfg := RetryConfig{}.withDefaults()

	assert.Equal(t, DefaultMaxAttempts, cfg.MaxAttempts)
	assert.Equal(t, DefaultInitialBackoff, cfg.InitialBackoff)
	assert.Equal(t, DefaultMaxBackoff, cfg.MaxBackoff)
	assert.InDelta(t, DefaultMultiplier, cfg.Multiplier, 0)
}
