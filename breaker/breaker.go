// Package breaker provides a circuit breaker for calls to a dependency that
// may fail or slow down. After a run of consecutive failures the breaker opens
// and rejects calls immediately with ErrOpen, so callers fail fast instead of
// waiting on a dependency that is down. After a cool-down it lets one trial
// call through (half-open): success closes it again, failure re-opens it.
package breaker

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrOpen is returned without calling the dependency while the breaker is open.
var ErrOpen = errors.New("circuit breaker open")

// State is the breaker's current mode.
type State int

const (
	// Closed passes every call through and counts consecutive failures.
	Closed State = iota
	// Open rejects every call with ErrOpen until the cool-down elapses.
	Open
	// HalfOpen lets a single trial call through to test recovery.
	HalfOpen
)

func (s State) String() string {
	switch s {
	case Closed:
		return "closed"
	case Open:
		return "open"
	case HalfOpen:
		return "half_open"
	}
	return "unknown"
}

// Default thresholds, used when a Config field is zero.
const (
	DefaultFailureThreshold = 5
	DefaultCoolDown         = 30 * time.Second
)

// Config tunes a Breaker. Zero values select the defaults.
type Config struct {
	// FailureThreshold is the number of consecutive failures that opens the breaker.
	FailureThreshold int
	// CoolDown is how long the breaker stays open before allowing a trial call.
	CoolDown time.Duration
	// IsFailure decides whether an error counts against the dependency. The
	// default counts every error except the caller's own cancellation.
	IsFailure func(error) bool
	// OnStateChange, when set, is called after every transition. It must not
	// block or call back into the breaker.
	OnStateChange func(from, to State)
	// Now is the clock; it defaults to time.Now and exists for tests.
	Now func() time.Time
}

// Breaker is a circuit breaker. It is safe for concurrent use.
type Breaker struct {
	cfg Config

	mu       sync.Mutex
	state    State
	failures int
	openedAt time.Time
	probing  bool
}

// New returns a closed Breaker configured by cfg.
func New(cfg Config) *Breaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = DefaultFailureThreshold
	}
	if cfg.CoolDown <= 0 {
		cfg.CoolDown = DefaultCoolDown
	}
	if cfg.IsFailure == nil {
		cfg.IsFailure = defaultIsFailure
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Breaker{cfg: cfg}
}

func defaultIsFailure(err error) bool {
	return err != nil && !errors.Is(err, context.Canceled)
}

// State reports the breaker's current mode.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// Health reports ErrOpen while the breaker is open and nil otherwise, matching
// the shape of a readiness check.
func (b *Breaker) Health(context.Context) error {
	if b.State() == Open {
		return ErrOpen
	}
	return nil
}

// Do calls fn unless the breaker is open, and records the outcome.
func (b *Breaker) Do(fn func() error) error {
	if err := b.admit(); err != nil {
		return err
	}
	err := fn()
	b.record(err)
	return err
}

// Execute is Do for a call that returns a value.
func Execute[T any](b *Breaker, fn func() (T, error)) (T, error) {
	var result T
	err := b.Do(func() error {
		var err error
		result, err = fn()
		return err
	})
	return result, err
}

// admit decides whether a call may proceed, moving an open breaker to
// half-open once its cool-down has elapsed.
func (b *Breaker) admit() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case Open:
		if b.cfg.Now().Sub(b.openedAt) < b.cfg.CoolDown {
			return ErrOpen
		}
		b.transition(HalfOpen)
		b.probing = true
		return nil
	case HalfOpen:
		if b.probing {
			return ErrOpen
		}
		b.probing = true
		return nil
	default:
		return nil
	}
}

func (b *Breaker) record(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	failed := b.cfg.IsFailure(err)
	if b.state == HalfOpen {
		b.probing = false
		if failed {
			b.open()
			return
		}
		b.failures = 0
		b.transition(Closed)
		return
	}

	if !failed {
		b.failures = 0
		return
	}
	b.failures++
	if b.failures >= b.cfg.FailureThreshold {
		b.open()
	}
}

func (b *Breaker) open() {
	b.openedAt = b.cfg.Now()
	b.failures = 0
	b.transition(Open)
}

func (b *Breaker) transition(to State) {
	from := b.state
	if from == to {
		return
	}
	b.state = to
	if b.cfg.OnStateChange != nil {
		b.cfg.OnStateChange(from, to)
	}
}
