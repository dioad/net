package dns

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAutoRefresher_Start_TicksRepeatedly(t *testing.T) {
	t.Parallel()

	var a autoRefresher
	var calls atomic.Int32
	done := make(chan struct{})

	a.start(5*time.Millisecond, func() {
		if calls.Add(1) == 3 {
			close(done)
		}
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("fetchFn was not called 3 times within 2s")
	}

	assert.GreaterOrEqual(t, calls.Load(), int32(3))
}

// TestAutoRefresher_Start_SecondCallIsNoOp verifies only one refresh loop
// runs per instance: a second start call, even with a much shorter period,
// must never run its fetchFn.
func TestAutoRefresher_Start_SecondCallIsNoOp(t *testing.T) {
	t.Parallel()

	var a autoRefresher
	var firstCalls atomic.Int32
	var secondCalls atomic.Int32

	firstStarted := make(chan struct{})
	a.start(5*time.Millisecond, func() {
		if firstCalls.Add(1) == 1 {
			close(firstStarted)
		}
	})
	<-firstStarted

	a.start(time.Millisecond, func() { secondCalls.Add(1) })

	time.Sleep(50 * time.Millisecond)

	assert.Zero(t, secondCalls.Load(), "second start call must be a no-op")
	assert.Positive(t, firstCalls.Load())
}
