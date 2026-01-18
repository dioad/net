package ratelimit

import (
	"fmt"
	"golang.org/x/time/rate"
)

// DemoReserveDelay demonstrates how to calculate the delay until next token
func DemoReserveDelay() {
	// Test 1: 1 req/sec, burst 1
	limiter := rate.NewLimiter(1, 1)
	limiter.Allow() // Use up the token
	r := limiter.Reserve()
	fmt.Printf("1 req/sec: Delay = %.0f seconds\n", r.Delay().Seconds())
	r.Cancel()

	// Test 2: 10 req/sec, burst 1
	limiter2 := rate.NewLimiter(10, 1)
	limiter2.Allow() // Use up the token
	r2 := limiter2.Reserve()
	fmt.Printf("10 req/sec: Delay = %.2f seconds\n", r2.Delay().Seconds())
	r2.Cancel()

	// Test 3: 0.5 req/sec (1 req per 2 seconds), burst 1
	limiter3 := rate.NewLimiter(0.5, 1)
	limiter3.Allow() // Use up the token
	r3 := limiter3.Reserve()
	fmt.Printf("0.5 req/sec: Delay = %.0f seconds\n", r3.Delay().Seconds())
	r3.Cancel()
}
