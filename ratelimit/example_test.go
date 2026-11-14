package ratelimit_test

import (
	"fmt"

	"github.com/dioad/net/ratelimit"
)

// Each principal has its own bucket: one that has used up its burst is
// refused while another is still allowed.
func ExampleRateLimiter_Allow() {
	limiter := ratelimit.NewRateLimiterWithOptions(
		// One request a minute, with a burst of two.
		ratelimit.WithRateLimiterStaticLimits(1.0/60, 2),
	)
	defer limiter.Stop()

	fmt.Println("alice:", limiter.Allow("alice"))
	fmt.Println("alice:", limiter.Allow("alice"))
	fmt.Println("alice:", limiter.Allow("alice"))
	fmt.Println("bob:", limiter.Allow("bob"))
	// Output:
	// alice: true
	// alice: true
	// alice: false
	// bob: true
}
