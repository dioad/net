package breaker

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Breaker_IsFailureClassification(t *testing.T) {
	errClient := errors.New("client error")
	b := New(Config{
		FailureThreshold: 2,
		IsFailure:        func(err error) bool { return !errors.Is(err, errClient) },
	})

	for range 5 {
		require.ErrorIs(t, b.Do(func() error { return errClient }), errClient)
	}
	assert.Equal(t, Closed, b.State(), "errors IsFailure rejects do not count")

	_ = b.Do(func() error { return errDown })
	_ = b.Do(func() error { return errDown })
	assert.Equal(t, Open, b.State())
}

func Test_Breaker_StateChangeSequence(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	var transitions [][2]State
	b := New(Config{
		FailureThreshold: 1,
		CoolDown:         time.Second,
		Now:              clock.Now,
		OnStateChange:    func(from, to State) { transitions = append(transitions, [2]State{from, to}) },
	})

	_ = b.Do(func() error { return errDown })
	clock.Advance(time.Second)
	_ = b.Do(func() error { return errDown }) // failed trial re-opens
	clock.Advance(time.Second)
	_ = b.Do(func() error { return nil }) // successful trial closes

	assert.Equal(t, [][2]State{
		{Closed, Open}, {Open, HalfOpen}, {HalfOpen, Open}, {Open, HalfOpen}, {HalfOpen, Closed},
	}, transitions)
}

func Test_Breaker_HalfOpenAdmitsOneTrialUnderConcurrency(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	b := New(Config{FailureThreshold: 1, CoolDown: time.Second, Now: clock.Now})
	_ = b.Do(func() error { return errDown })
	clock.Advance(time.Second)

	release := make(chan struct{})
	var admitted, rejected atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			err := b.Do(func() error {
				admitted.Add(1)
				<-release

				return nil
			})
			if errors.Is(err, ErrOpen) {
				rejected.Add(1)
			}
		})
	}

	require.Eventually(t, func() bool { return rejected.Load() == 19 }, 5*time.Second, time.Millisecond)
	close(release)
	wg.Wait()

	assert.Equal(t, int32(1), admitted.Load())
	assert.Equal(t, Closed, b.State())
}

func Test_State_String(t *testing.T) {
	assert.Equal(t, "closed", Closed.String())
	assert.Equal(t, "open", Open.String())
	assert.Equal(t, "half_open", HalfOpen.String())
	assert.Equal(t, "unknown", State(42).String())
}
