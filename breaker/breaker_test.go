package breaker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errDown = errors.New("dependency down")

// fakeClock is a manually advanced clock for driving the cool-down.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newTestBreaker(t *testing.T, threshold int) (*Breaker, *fakeClock, *[]State) {
	t.Helper()
	clock := &fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	var transitions []State
	b := New(Config{
		FailureThreshold: threshold,
		CoolDown:         time.Minute,
		Now:              clock.Now,
		OnStateChange:    func(_, to State) { transitions = append(transitions, to) },
	})
	return b, clock, &transitions
}

func fail(b *Breaker, n int) {
	for range n {
		_ = b.Do(func() error { return errDown })
	}
}

func TestBreaker_OpensAfterConsecutiveFailures(t *testing.T) {
	t.Parallel()
	b, _, _ := newTestBreaker(t, 3)

	fail(b, 2)
	assert.Equal(t, Closed, b.State(), "below the threshold the breaker stays closed")

	fail(b, 1)
	assert.Equal(t, Open, b.State())
}

func TestBreaker_SuccessResetsTheFailureCount(t *testing.T) {
	t.Parallel()
	b, _, _ := newTestBreaker(t, 3)

	fail(b, 2)
	require.NoError(t, b.Do(func() error { return nil }))
	fail(b, 2)

	assert.Equal(t, Closed, b.State(), "failures must be consecutive to open the breaker")
}

func TestBreaker_OpenRejectsWithoutCallingTheDependency(t *testing.T) {
	t.Parallel()
	b, _, _ := newTestBreaker(t, 1)
	fail(b, 1)

	called := false
	err := b.Do(func() error { called = true; return nil })

	require.ErrorIs(t, err, ErrOpen)
	assert.False(t, called)
}

func TestBreaker_HalfOpenTrialDecidesTheNextState(t *testing.T) {
	t.Parallel()

	t.Run("successful trial closes the breaker", func(t *testing.T) {
		t.Parallel()
		b, clock, transitions := newTestBreaker(t, 1)
		fail(b, 1)
		clock.Advance(time.Minute)

		require.NoError(t, b.Do(func() error { return nil }))

		assert.Equal(t, Closed, b.State())
		assert.Equal(t, []State{Open, HalfOpen, Closed}, *transitions)
	})

	t.Run("failed trial re-opens the breaker for another cool-down", func(t *testing.T) {
		t.Parallel()
		b, clock, _ := newTestBreaker(t, 1)
		fail(b, 1)
		clock.Advance(time.Minute)

		fail(b, 1)

		assert.Equal(t, Open, b.State())
		assert.ErrorIs(t, b.Do(func() error { return nil }), ErrOpen)
	})

	t.Run("only one trial call runs at a time", func(t *testing.T) {
		t.Parallel()
		b, clock, _ := newTestBreaker(t, 1)
		fail(b, 1)
		clock.Advance(time.Minute)

		var second error
		require.NoError(t, b.Do(func() error {
			second = b.Do(func() error { return nil })
			return nil
		}))

		assert.ErrorIs(t, second, ErrOpen, "a concurrent call during the trial is rejected")
	})
}

func TestBreaker_StaysOpenUntilTheCoolDownElapses(t *testing.T) {
	t.Parallel()
	b, clock, _ := newTestBreaker(t, 1)
	fail(b, 1)

	clock.Advance(59 * time.Second)

	assert.ErrorIs(t, b.Do(func() error { return nil }), ErrOpen)
}

func TestBreaker_CallerCancellationIsNotAFailure(t *testing.T) {
	t.Parallel()
	b, _, _ := newTestBreaker(t, 1)

	_ = b.Do(func() error { return context.Canceled })

	assert.Equal(t, Closed, b.State())
}

func TestExecute_ReturnsTheValue(t *testing.T) {
	t.Parallel()
	b := New(Config{})

	got, err := Execute(b, func() (int, error) { return 42, nil })

	require.NoError(t, err)
	assert.Equal(t, 42, got)
}

func TestNew_AppliesDefaults(t *testing.T) {
	t.Parallel()
	b := New(Config{})

	assert.Equal(t, DefaultFailureThreshold, b.cfg.FailureThreshold)
	assert.Equal(t, DefaultCoolDown, b.cfg.CoolDown)
	assert.Equal(t, Closed, b.State())
}

func TestBreaker_IsSafeForConcurrentUse(t *testing.T) {
	t.Parallel()
	b := New(Config{FailureThreshold: 3})

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Go(func() {
			_ = b.Do(func() error {
				if i%2 == 0 {
					return errDown
				}
				return nil
			})
		})
	}
	wg.Wait()

	assert.Contains(t, []State{Closed, Open, HalfOpen}, b.State())
}

func TestBreaker_HealthReportsAnOpenBreaker(t *testing.T) {
	t.Parallel()
	b, _, _ := newTestBreaker(t, 1)
	require.NoError(t, b.Health(context.Background()))

	fail(b, 1)

	assert.ErrorIs(t, b.Health(context.Background()), ErrOpen)
}
