package ratelimit

import (
	"bytes"
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter_Allow(t *testing.T) {
	logger := zerolog.New(zerolog.NewConsoleWriter())
	rl := NewRateLimiter(1, 2, logger)
	defer rl.Stop()

	// Initial burst
	assert.True(t, rl.Allow("user1"))
	assert.True(t, rl.Allow("user1"))
	// Exceeded burst
	assert.False(t, rl.Allow("user1"))

	// Different principal should have its own limit
	assert.True(t, rl.Allow("user2"))
	assert.True(t, rl.Allow("user2"))
	assert.False(t, rl.Allow("user2"))
}

func TestRateLimiter_Cleanup(t *testing.T) {
	logger := zerolog.New(zerolog.NewConsoleWriter())
	rl := NewRateLimiterWithConfig(10, 10, 10*time.Millisecond, 20*time.Millisecond, logger)
	defer rl.Stop()

	// Add some limiters
	rl.Allow("user1")
	rl.Allow("user2")

	rl.mu.Lock()
	assert.Len(t, rl.limiters, 2)
	rl.mu.Unlock()

	// Wait for TTL to pass for user1 but not user2?
	// Actually easier to just wait for both.
	time.Sleep(30 * time.Millisecond)

	// This call should trigger cleanup because enough time passed since lastCleanup (initialized to time.Now())
	// and enough time passed since user1/user2 were last used.
	rl.Allow("user3")

	rl.mu.Lock()
	// user1 and user2 should be gone, user3 should be there
	assert.Len(t, rl.limiters, 1)
	assert.Contains(t, rl.limiters, "user3")
	assert.NotContains(t, rl.limiters, "user1")
	assert.NotContains(t, rl.limiters, "user2")
	rl.mu.Unlock()
}

func TestRateLimiter_cleanupExpiredLimiters(t *testing.T) {
	t.Run("logs and removes entries past the stale TTL", func(t *testing.T) {
		var buf bytes.Buffer
		logger := zerolog.New(&buf)
		rl := NewRateLimiterWithConfig(1, 1, time.Hour, 5*time.Millisecond, logger)
		defer rl.Stop()

		rl.Allow("stale-user")
		rl.Allow("fresh-user")

		time.Sleep(20 * time.Millisecond)

		// Refresh fresh-user so only stale-user is past the TTL.
		rl.Allow("fresh-user")

		rl.cleanupExpiredLimiters()

		rl.mu.RLock()
		_, staleStillPresent := rl.limiters["stale-user"]
		_, freshStillPresent := rl.limiters["fresh-user"]
		rl.mu.RUnlock()

		assert.False(t, staleStillPresent, "the entry past the stale TTL should be removed")
		assert.True(t, freshStillPresent, "the recently-used entry should be kept")
		assert.Contains(t, buf.String(), `"removed_limiters":1`)
		assert.Contains(t, buf.String(), `"remaining_limiters":1`)
		assert.Contains(t, buf.String(), "cleaned up stale rate limiters")
	})

	t.Run("does not log when nothing is stale", func(t *testing.T) {
		var buf bytes.Buffer
		logger := zerolog.New(&buf)
		rl := NewRateLimiterWithConfig(1, 1, time.Hour, time.Hour, logger)
		defer rl.Stop()

		rl.Allow("user1")
		rl.cleanupExpiredLimiters()

		assert.Empty(t, buf.String(), "cleanupExpiredLimiters should not log when no entries were removed")
	})
}

func TestRateLimiter_Refill(t *testing.T) {
	logger := zerolog.New(zerolog.NewConsoleWriter())
	// 10 tokens per second, burst of 1
	rl := NewRateLimiter(10, 1, logger)
	defer rl.Stop()

	assert.True(t, rl.Allow("user1"))
	assert.False(t, rl.Allow("user1"))

	// Wait for refill (0.1s for 1 token)
	time.Sleep(110 * time.Millisecond)
	assert.True(t, rl.Allow("user1"))
}

func TestNewRateLimiterWithConfig_ClampsInvalidInputs(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("negative rps clamps to zero, small positive rps is preserved", func(t *testing.T) {
		clamped := NewRateLimiterWithConfig(-0.5, 5, time.Minute, time.Minute, logger)
		defer clamped.Stop()
		assert.Equal(t, 0.0, clamped.requestsPerSecond) //nolint:testifylint // exact clamp target, not a computed float

		preserved := NewRateLimiterWithConfig(0.5, 5, time.Minute, time.Minute, logger)
		defer preserved.Stop()
		assert.Equal(t, 0.5, preserved.requestsPerSecond) //nolint:testifylint // exact passthrough of the input literal, not a computed float
	})

	t.Run("negative burst clamps to zero", func(t *testing.T) {
		rl := NewRateLimiterWithConfig(1, -1, time.Minute, time.Minute, logger)
		defer rl.Stop()

		assert.Equal(t, 0, rl.burst)
	})

	t.Run("negative rps logs a warning naming the rejected value", func(t *testing.T) {
		var buf bytes.Buffer
		rl := NewRateLimiterWithConfig(-0.5, 5, time.Minute, time.Minute, zerolog.New(&buf))
		defer rl.Stop()

		assert.Contains(t, buf.String(), "requestsPerSecond")
		assert.Contains(t, buf.String(), "-0.5")
	})

	t.Run("negative burst logs a warning naming the rejected value", func(t *testing.T) {
		var buf bytes.Buffer
		rl := NewRateLimiterWithConfig(1, -3, time.Minute, time.Minute, zerolog.New(&buf))
		defer rl.Stop()

		assert.Contains(t, buf.String(), "burst")
		assert.Contains(t, buf.String(), "-3")
	})

	t.Run("valid inputs log no warning", func(t *testing.T) {
		var buf bytes.Buffer
		rl := NewRateLimiterWithConfig(1, 1, time.Minute, time.Minute, zerolog.New(&buf))
		defer rl.Stop()

		assert.Empty(t, buf.String())
	})

	t.Run("non-positive cleanup interval falls back to default, tiny positive value is preserved", func(t *testing.T) {
		clamped := NewRateLimiterWithConfig(1, 1, 0, time.Minute, logger)
		defer clamped.Stop()
		assert.Equal(t, 5*time.Minute, clamped.cleanupInterval)

		preserved := NewRateLimiterWithConfig(1, 1, time.Nanosecond, time.Minute, logger)
		assert.Equal(t, time.Nanosecond, preserved.cleanupInterval)
		preserved.Stop()
	})

	t.Run("non-positive stale TTL falls back to default, tiny positive value is preserved", func(t *testing.T) {
		clamped := NewRateLimiterWithConfig(1, 1, time.Minute, 0, logger)
		defer clamped.Stop()
		assert.Equal(t, 30*time.Minute, clamped.staleTTL)

		preserved := NewRateLimiterWithConfig(1, 1, time.Minute, time.Nanosecond, logger)
		assert.Equal(t, time.Nanosecond, preserved.staleTTL)
		preserved.Stop()
	})
}

func TestNewRateLimiterWithContextAndConfig_ClampsInvalidInputs(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("negative rps clamps to zero, small positive rps is preserved", func(t *testing.T) {
		clamped := NewRateLimiterWithContextAndConfig(context.Background(), -0.5, 5, time.Minute, time.Minute, logger)
		defer clamped.Stop()
		assert.Equal(t, 0.0, clamped.requestsPerSecond) //nolint:testifylint // exact clamp target, not a computed float

		preserved := NewRateLimiterWithContextAndConfig(context.Background(), 0.5, 5, time.Minute, time.Minute, logger)
		defer preserved.Stop()
		assert.Equal(t, 0.5, preserved.requestsPerSecond) //nolint:testifylint // exact passthrough of the input literal, not a computed float
	})

	t.Run("negative burst clamps to zero", func(t *testing.T) {
		rl := NewRateLimiterWithContextAndConfig(context.Background(), 1, -1, time.Minute, time.Minute, logger)
		defer rl.Stop()

		assert.Equal(t, 0, rl.burst)
	})

	t.Run("non-positive cleanup interval falls back to default, tiny positive value is preserved", func(t *testing.T) {
		clamped := NewRateLimiterWithContextAndConfig(context.Background(), 1, 1, 0, time.Minute, logger)
		defer clamped.Stop()
		assert.Equal(t, 5*time.Minute, clamped.cleanupInterval)

		preserved := NewRateLimiterWithContextAndConfig(context.Background(), 1, 1, time.Nanosecond, time.Minute, logger)
		assert.Equal(t, time.Nanosecond, preserved.cleanupInterval)
		preserved.Stop()
	})

	t.Run("non-positive stale TTL falls back to default, tiny positive value is preserved", func(t *testing.T) {
		clamped := NewRateLimiterWithContextAndConfig(context.Background(), 1, 1, time.Minute, 0, logger)
		defer clamped.Stop()
		assert.Equal(t, 30*time.Minute, clamped.staleTTL)

		preserved := NewRateLimiterWithContextAndConfig(context.Background(), 1, 1, time.Minute, time.Nanosecond, logger)
		assert.Equal(t, time.Nanosecond, preserved.staleTTL)
		preserved.Stop()
	})
}

func TestNewRateLimiterWithContextAndConfig_StartsBackgroundCleanup(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	rl := NewRateLimiterWithContextAndConfig(context.Background(), 10, 10, 20*time.Millisecond, 10*time.Millisecond, logger)
	defer rl.Stop()

	rl.Allow("user1")

	require.Eventually(t, func() bool {
		rl.mu.RLock()
		defer rl.mu.RUnlock()

		return len(rl.limiters) == 0
	}, 500*time.Millisecond, 10*time.Millisecond, "background cleanup goroutine should remove the stale limiter")
}

func TestNewRateLimiterWithSourceAndConfig_ClampsInvalidInputs(t *testing.T) {
	logger := zerolog.Nop()
	source := &StaticRateLimitSource{RequestsPerSecond: 1, Burst: 1}

	t.Run("non-positive cleanup interval falls back to default, tiny positive value is preserved", func(t *testing.T) {
		clamped := NewRateLimiterWithSourceAndConfig(source, 0, time.Minute, logger)
		defer clamped.Stop()
		assert.Equal(t, 5*time.Minute, clamped.cleanupInterval)

		preserved := NewRateLimiterWithSourceAndConfig(source, time.Nanosecond, time.Minute, logger)
		assert.Equal(t, time.Nanosecond, preserved.cleanupInterval)
		preserved.Stop()
	})

	t.Run("non-positive stale TTL falls back to default, tiny positive value is preserved", func(t *testing.T) {
		clamped := NewRateLimiterWithSourceAndConfig(source, time.Minute, 0, logger)
		defer clamped.Stop()
		assert.Equal(t, 30*time.Minute, clamped.staleTTL)

		preserved := NewRateLimiterWithSourceAndConfig(source, time.Minute, time.Nanosecond, logger)
		assert.Equal(t, time.Nanosecond, preserved.staleTTL)
		preserved.Stop()
	})
}

func TestNewRateLimiterWithSourceAndConfig_StartsBackgroundCleanup(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	source := &StaticRateLimitSource{RequestsPerSecond: 1000, Burst: 1000}
	rl := NewRateLimiterWithSourceAndConfig(source, 20*time.Millisecond, 10*time.Millisecond, logger)
	defer rl.Stop()

	rl.Allow("user1")

	require.Eventually(t, func() bool {
		rl.mu.RLock()
		defer rl.mu.RUnlock()

		return len(rl.limiters) == 0
	}, 500*time.Millisecond, 10*time.Millisecond, "background cleanup goroutine should remove the stale limiter")
}

type mockSource struct {
	limits map[string]struct {
		rps   float64
		burst int
	}
}

func (m *mockSource) GetLimit(principal string) (float64, int, bool) {
	l, ok := m.limits[principal]
	if !ok {
		return 0, 0, false
	}

	return l.rps, l.burst, true
}

func TestRateLimiter_WithSource(t *testing.T) {
	logger := zerolog.Nop()
	source := &mockSource{
		limits: map[string]struct {
			rps   float64
			burst int
		}{
			"premium": {rps: 1000, burst: 1000},
			"free":    {rps: 1, burst: 1},
		},
	}
	rl := NewRateLimiterWithSource(source, logger)
	defer rl.Stop()

	// Premium user
	for range 50 {
		assert.True(t, rl.Allow("premium"))
	}

	// Free user
	assert.True(t, rl.Allow("free"))
	assert.False(t, rl.Allow("free"))
}

func TestRateLimiter_WithSourceAndFallback(t *testing.T) {
	logger := zerolog.Nop()
	source := &mockSource{
		limits: map[string]struct {
			rps   float64
			burst int
		}{
			"premium": {rps: 1000, burst: 1000},
			"free":    {rps: 1, burst: 1},
		},
	}

	// Create a rate limiter with fallback limits first, then we'll set source
	// This approach is not ideal but demonstrates fallback behavior
	// In production, consider having the source always return ok=true
	rl := NewRateLimiterWithConfig(5, 5, 5*time.Minute, 30*time.Minute, logger)
	defer rl.Stop()

	// SetLimitSource is safe to call after construction, including
	// concurrently with in-flight Allow() calls.
	rl.SetLimitSource(source)

	// Premium user should use source limits
	for range 50 {
		assert.True(t, rl.Allow("premium"))
	}

	// Free user should use source limits
	assert.True(t, rl.Allow("free"))
	assert.False(t, rl.Allow("free"))

	// Unknown user should use fallback limits (5 rps, 5 burst)
	for range 5 {
		assert.True(t, rl.Allow("unknown"))
	}
	assert.False(t, rl.Allow("unknown"))
}

func TestRateLimiter_WarnsWhenSourceFallbackIsZeroValue(t *testing.T) {
	source := &mockSource{
		limits: map[string]struct {
			rps   float64
			burst int
		}{
			"known": {rps: 1000, burst: 1000},
		},
	}

	t.Run("unmatched principal with no static fallback logs a warning once", func(t *testing.T) {
		var buf bytes.Buffer
		rl := NewRateLimiterWithSource(source, zerolog.New(&buf))
		defer rl.Stop()

		assert.False(t, rl.Allow("unknown"), "with no static fallback configured, an unmatched principal must be denied")
		assert.Contains(t, buf.String(), "RateLimitSource returned ok=false")
		assert.Contains(t, buf.String(), `"principal":"unknown"`)

		// A second call for the same (or another) unmatched principal must
		// not log again -- the warning fires once per RateLimiter.
		buf.Reset()
		rl.Allow("unknown")
		rl.Allow("another-unknown")
		assert.Empty(t, buf.String(), "the zero-value-fallback warning must only be logged once per RateLimiter")
	})

	t.Run("unmatched principal with an explicit static fallback logs nothing", func(t *testing.T) {
		var buf bytes.Buffer
		rl := NewRateLimiterWithOptions(
			WithRateLimiterSource(source),
			WithRateLimiterStaticLimits(5, 5),
			WithRateLimiterLogger(zerolog.New(&buf)),
		)
		defer rl.Stop()

		assert.True(t, rl.Allow("unknown"), "an explicit static fallback must still be usable for unmatched principals")
		assert.Empty(t, buf.String(), "no warning should be logged when a static fallback is explicitly configured")
	})

	t.Run("matched principal never triggers the fallback warning", func(t *testing.T) {
		var buf bytes.Buffer
		rl := NewRateLimiterWithSource(source, zerolog.New(&buf))
		defer rl.Stop()

		rl.Allow("known")
		assert.Empty(t, buf.String())
	})
}

func TestRateLimiter_ConcurrentLimitSourceMutation(_ *testing.T) {
	// Regression test for a data race: SetLimitSource must be safe to call
	// concurrently with Allow() from other goroutines -- the natural way to
	// hot-reload limits for something documented as "dynamic". Run with
	// -race; a direct field assignment to an unsynchronized LimitSource
	// used to trip the race detector here.
	logger := zerolog.Nop()
	rl := NewRateLimiterWithConfig(5, 5, 5*time.Minute, 30*time.Minute, logger)
	defer rl.Stop()

	source := &mockSource{limits: map[string]struct {
		rps   float64
		burst int
	}{"user": {rps: 10, burst: 10}}}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 200 {
			rl.Allow("user")
		}
	}()
	go func() {
		defer wg.Done()
		for range 200 {
			rl.SetLimitSource(source)
		}
	}()
	wg.Wait()
}

func TestRateLimiter_DynamicUpdate(t *testing.T) {
	logger := zerolog.Nop()
	source := &mockSource{
		limits: map[string]struct {
			rps   float64
			burst int
		}{
			"user1": {rps: 1, burst: 1},
		},
	}
	rl := NewRateLimiterWithSource(source, logger)
	defer rl.Stop()

	assert.True(t, rl.Allow("user1"))
	assert.False(t, rl.Allow("user1"))

	// Update limits in source - significantly increase RPS and burst
	source.limits["user1"] = struct {
		rps   float64
		burst int
	}{rps: 10000, burst: 10000}

	// Call Allow once to trigger the update in the rate limiter entry.
	// It will likely still return false because it hasn't refilled tokens yet.
	rl.Allow("user1")

	// Wait a bit for tokens to refill at the new high rate.
	time.Sleep(10 * time.Millisecond)

	assert.True(t, rl.Allow("user1"), "Should allow after limit increase and refill")
}

func TestRateLimiter_Allow_UpdatesBurstOnLimitChange(t *testing.T) {
	logger := zerolog.Nop()
	source := &mockSource{
		limits: map[string]struct {
			rps   float64
			burst int
		}{
			"user1": {rps: 1000, burst: 1},
		},
	}
	rl := NewRateLimiterWithSource(source, logger)
	defer rl.Stop()

	rl.Allow("user1")

	source.limits["user1"] = struct {
		rps   float64
		burst int
	}{rps: 1000, burst: 5}

	rl.Allow("user1")

	rl.mu.RLock()
	burst := rl.limiters["user1"].limiter.Burst()
	rl.mu.RUnlock()

	assert.Equal(t, 5, burst, "the underlying limiter's burst should track a changed source limit")
}

func TestRateLimiter_Allow_RefreshesLastUsedOnEachCall(t *testing.T) {
	logger := zerolog.Nop()
	rl := NewRateLimiter(1000, 1000, logger)
	defer rl.Stop()

	rl.Allow("user1")
	rl.mu.RLock()
	first := rl.limiters["user1"].lastUsed
	rl.mu.RUnlock()

	time.Sleep(5 * time.Millisecond)
	rl.Allow("user1")
	rl.mu.RLock()
	second := rl.limiters["user1"].lastUsed
	rl.mu.RUnlock()

	assert.True(t, second.After(first), "lastUsed should advance on every Allow call, not just entry creation")
}

func TestStaticRateLimitSource(t *testing.T) {
	source := &StaticRateLimitSource{RequestsPerSecond: 10, Burst: 20}
	rps, burst, ok := source.GetLimit("any")
	assert.True(t, ok)
	assert.Equal(t, 10.0, rps) //nolint:testifylint // exact passthrough of the struct literal field, not a computed float
	assert.Equal(t, 20, burst)
}

func BenchmarkRateLimiter_Allow_Sequential(b *testing.B) {
	logger := zerolog.Nop()
	rl := NewRateLimiter(1000000, 1000000, logger) // High limits to focus on lock contention
	defer rl.Stop()

	b.ResetTimer()
	for range b.N {
		rl.Allow("user1")
	}
}

func BenchmarkRateLimiter_Allow_Parallel(b *testing.B) {
	logger := zerolog.Nop()
	rl := NewRateLimiter(1000000, 1000000, logger) // High limits to focus on lock contention
	defer rl.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rl.Allow("user1")
		}
	})
}

func BenchmarkRateLimiter_Allow_ParallelMultiPrincipal(b *testing.B) {
	logger := zerolog.Nop()
	rl := NewRateLimiter(1000000, 1000000, logger) // High limits to focus on lock contention
	defer rl.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Simulate multiple principals to test map access patterns
			principal := "user" + strconv.Itoa(i%10)
			rl.Allow(principal)
			i++
		}
	})
}

func TestRateLimiter_RetryAfter(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name              string
		requestsPerSecond float64
		burst             int
		expectedSeconds   float64
		tolerance         float64 // Allow some tolerance for timing
	}{
		{
			name:              "1 request per second",
			requestsPerSecond: 1,
			burst:             1,
			expectedSeconds:   1.0,
			tolerance:         0.1,
		},
		{
			name:              "10 requests per second",
			requestsPerSecond: 10,
			burst:             1,
			expectedSeconds:   0.1,
			tolerance:         0.01,
		},
		{
			name:              "0.5 requests per second",
			requestsPerSecond: 0.5,
			burst:             1,
			expectedSeconds:   2.0,
			tolerance:         0.1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(tt.requestsPerSecond, tt.burst, logger)
			defer rl.Stop()

			// Use up the burst
			rl.Allow("user1")

			// Now check retry-after time
			retryAfter := rl.RetryAfter("user1")
			actualSeconds := retryAfter.Seconds()

			assert.InDelta(t, tt.expectedSeconds, actualSeconds, tt.tolerance,
				"RetryAfter should return approximately %.2f seconds for %.1f req/sec",
				tt.expectedSeconds, tt.requestsPerSecond)
		})
	}
}

func TestRateLimiter_RetryAfter_NoEntry(t *testing.T) {
	logger := zerolog.Nop()
	rl := NewRateLimiter(1, 1, logger)
	defer rl.Stop()

	// RetryAfter for a principal that hasn't been seen should return 0
	retryAfter := rl.RetryAfter("unknown_user")
	assert.Equal(t, time.Duration(0), retryAfter)
}

func TestRateLimiter_Stop(t *testing.T) {
	logger := zerolog.Nop()
	rl := NewRateLimiter(1, 1, logger)

	// Use the rate limiter
	assert.True(t, rl.Allow("user1"))

	// Stop should gracefully shut down the background goroutine
	rl.Stop()

	// Verify the context is cancelled
	select {
	case <-rl.ctx.Done():
		// Expected - context should be done
	default:
		t.Fatal("Context should be done after Stop()")
	}
}

func TestRateLimiter_StopMultipleTimes(t *testing.T) {
	logger := zerolog.Nop()
	rl := NewRateLimiter(1, 1, logger)

	// Use the rate limiter
	assert.True(t, rl.Allow("user1"))

	// Stop multiple times should be safe
	rl.Stop()
	rl.Stop()
	rl.Stop()

	// Verify the context is cancelled
	select {
	case <-rl.ctx.Done():
		// Expected - context should be done
	default:
		t.Fatal("Context should be done after Stop()")
	}
}

// TestRateLimiter_ZeroValueIsSafeToUse exercises the exported type's zero
// value directly (var rl RateLimiter), the natural first attempt for an
// exported struct type without knowing a constructor is required. It must
// not panic -- a nil limiters map or nil cancel func should degrade
// gracefully (no background cleanup, Stop a no-op) rather than crash on
// first use.
func TestRateLimiter_ZeroValueIsSafeToUse(t *testing.T) {
	var rl RateLimiter

	// The zero value has 0 rps/0 burst, so Allow denies every request --
	// that's fine; the point of this test is that it doesn't panic.
	assert.NotPanics(t, func() {
		rl.Allow("user1")
	})

	assert.NotPanics(t, func() {
		rl.Stop()
	})
}

func TestRateLimiter_WithContext(t *testing.T) {
	logger := zerolog.Nop()
	ctx, cancel := context.WithCancel(context.Background())

	rl := NewRateLimiterWithContext(ctx, 1, 1, logger)

	// Use the rate limiter
	assert.True(t, rl.Allow("user1"))

	// Cancel the context
	cancel()

	// Wait for the background goroutine to exit
	rl.wg.Wait()

	// Verify the context is cancelled
	select {
	case <-rl.ctx.Done():
		// Expected - context should be done
	default:
		t.Fatal("Context should be done after parent context cancellation")
	}
}

func TestRateLimiter_WithContextCancellation(t *testing.T) {
	logger := zerolog.Nop()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rl := NewRateLimiterWithContextAndConfig(ctx, 10, 10, 50*time.Millisecond, 30*time.Millisecond, logger)

	// Add some limiters
	rl.Allow("user1")
	rl.Allow("user2")

	rl.mu.Lock()
	assert.Len(t, rl.limiters, 2)
	rl.mu.Unlock()

	// Cancel the context
	cancel()

	// Wait for the background goroutine to stop
	rl.wg.Wait()

	// Verify the context is done
	select {
	case <-rl.ctx.Done():
		// Expected
	default:
		t.Fatal("Context should be done after cancellation")
	}
}

func TestRateLimiter_BackgroundCleanup(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	rl := NewRateLimiterWithConfig(10, 10, 50*time.Millisecond, 30*time.Millisecond, logger)
	defer rl.Stop()

	// Add some limiters
	rl.Allow("user1")
	rl.Allow("user2")

	rl.mu.Lock()
	assert.Len(t, rl.limiters, 2)
	rl.mu.Unlock()

	// Wait for TTL to pass
	time.Sleep(40 * time.Millisecond)

	// Poll for cleanup to complete with timeout
	maxWait := 200 * time.Millisecond
	pollInterval := 10 * time.Millisecond
	startTime := time.Now()

	for time.Since(startTime) < maxWait {
		rl.mu.Lock()
		count := len(rl.limiters)
		rl.mu.Unlock()

		if count == 0 {
			// Cleanup succeeded
			return
		}
		time.Sleep(pollInterval)
	}

	// If we get here, cleanup didn't happen in time
	rl.mu.Lock()
	count := len(rl.limiters)
	rl.mu.Unlock()

	t.Fatalf("Expected limiters to be cleaned up, but found %d limiters after %v", count, maxWait)
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	rl := NewRateLimiter(1000000, 1000000, logger)
	defer rl.Stop()

	const numGoroutines = 100
	const numIterations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Run multiple goroutines concurrently accessing the rate limiter
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			principal := "user" + strconv.Itoa(id%10)
			for range numIterations {
				rl.Allow(principal)
			}
		}(i)
	}

	wg.Wait()

	// Verify no panics occurred and limiters were created
	rl.mu.RLock()
	assert.NotEmpty(t, rl.limiters)
	assert.LessOrEqual(t, len(rl.limiters), 10) // Max 10 unique principals
	rl.mu.RUnlock()
}
