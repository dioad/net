package breaker_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dioad/lifecycle"

	"github.com/dioad/net/breaker"
)

// A breaker guards calls to a dependency. Its Health method has the shape of
// a lifecycle.CheckFunc, so registering it as a degraded check reports the
// open breaker without taking the service out of rotation.
func ExampleBreaker_Health() {
	b := breaker.New(breaker.Config{
		FailureThreshold: 2,
		CoolDown:         time.Minute,
		// Each service keeps its own metric names.
		OnStateChange: func(from, to breaker.State) { fmt.Println(from, "->", to) },
	})

	readiness := lifecycle.NewReadiness()
	readiness.AddCheck("payments", b.Health, lifecycle.WithDegraded())

	errDown := errors.New("down")
	for range 2 {
		_ = b.Do(func() error { return errDown })
	}
	fmt.Println(b.Health(context.Background()))

	// Output:
	// closed -> open
	// circuit breaker open
}

// Retry composes with a breaker: ErrOpen is not retried, so a retry loop
// stops as soon as the breaker opens.
func ExampleRetry() {
	b := breaker.New(breaker.Config{FailureThreshold: 1})
	attempts := 0

	err := breaker.Retry(context.Background(), breaker.RetryConfig{
		MaxAttempts:    5,
		InitialBackoff: time.Millisecond,
	}, func(context.Context) error {
		return b.Do(func() error {
			attempts++

			return errors.New("down")
		})
	})

	fmt.Println(attempts, errors.Is(err, breaker.ErrOpen))

	// Output:
	// 1 true
}
