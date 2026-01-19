package ratelimithttp_test

import (
	"fmt"
	"net/http"

	diohttp "github.com/dioad/net/http"
	"github.com/rs/zerolog"
)

// Example demonstrates HTTP rate limiting for a specific principal.
func Example() {
	logger := zerolog.New(nil)

	// Create rate limiter (1 request per second, burst of 5)
	limiter := diohttp.NewRateLimiter(1.0, 5, logger)

	// Create a simple handler
	myHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Request processed")
	})

	// Use as middleware for a specific principal
	handler := limiter.Middleware("user1")(myHandler)

	fmt.Printf("Handler configured with rate limiting: %T\n", handler)
	// Output: Handler configured with rate limiting: http.HandlerFunc
}
