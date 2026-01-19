package ratelimitdynamic_test

import (
	"fmt"
	"net/http"

	diohttp "github.com/dioad/net/http"
	"github.com/rs/zerolog"
)

// mySource implements RateLimitSource for dynamic rate limiting.
type mySource struct{}

func (s *mySource) GetLimit(principal string) (float64, int, bool) {
	if principal == "premium" {
		return 100.0, 100, true
	}
	return 1.0, 5, true
}

// Example demonstrates dynamic rate limiting with custom source.
func Example() {
	logger := zerolog.New(nil)

	// Create rate limiter with custom source
	limiter := diohttp.NewRateLimiterWithSource(&mySource{}, logger)

	// Create a simple handler
	myHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Request processed")
	})

	// Use as middleware
	handler := limiter.Middleware("premium")(myHandler)

	fmt.Printf("Handler configured with dynamic rate limiting: %T\n", handler)
	// Output: Handler configured with dynamic rate limiting: http.HandlerFunc
}
