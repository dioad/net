package ratelimit

import (
	"strconv"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_Allow(t *testing.T) {
	logger := zerolog.New(zerolog.NewConsoleWriter())
	rl := NewRateLimiter(1, 2, logger)

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
	rl := NewRateLimiter(10, 10, logger)
	rl.CleanupInterval = 10 * time.Millisecond
	rl.StaleTTL = 20 * time.Millisecond

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

func TestRateLimiter_Refill(t *testing.T) {
	logger := zerolog.New(zerolog.NewConsoleWriter())
	// 10 tokens per second, burst of 1
	rl := NewRateLimiter(10, 1, logger)

	assert.True(t, rl.Allow("user1"))
	assert.False(t, rl.Allow("user1"))

	// Wait for refill (0.1s for 1 token)
	time.Sleep(110 * time.Millisecond)
	assert.True(t, rl.Allow("user1"))
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
	// Default fallback if no principal match (ok=false)
	rl.RequestsPerSecond = 5
	rl.Burst = 5

	// Premium user
	for i := 0; i < 50; i++ {
		assert.True(t, rl.Allow("premium"))
	}

	// Free user
	assert.True(t, rl.Allow("free"))
	assert.False(t, rl.Allow("free"))

	// Fallback user
	for i := 0; i < 5; i++ {
		assert.True(t, rl.Allow("unknown"))
	}
	assert.False(t, rl.Allow("unknown"))
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

func TestStaticRateLimitSource(t *testing.T) {
	source := &StaticRateLimitSource{RequestsPerSecond: 10, Burst: 20}
	rps, burst, ok := source.GetLimit("any")
	assert.True(t, ok)
	assert.Equal(t, 10.0, rps)
	assert.Equal(t, 20, burst)
}

func BenchmarkRateLimiter_Allow_Sequential(b *testing.B) {
	logger := zerolog.Nop()
	rl := NewRateLimiter(1000000, 1000000, logger) // High limits to focus on lock contention

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Allow("user1")
	}
}

func BenchmarkRateLimiter_Allow_Parallel(b *testing.B) {
	logger := zerolog.Nop()
	rl := NewRateLimiter(1000000, 1000000, logger) // High limits to focus on lock contention

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

	// RetryAfter for a principal that hasn't been seen should return 0
	retryAfter := rl.RetryAfter("unknown_user")
	assert.Equal(t, time.Duration(0), retryAfter)
}
