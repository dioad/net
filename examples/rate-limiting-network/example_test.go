package ratelimitnetwork_test

import (
	"fmt"
	"net"

	"github.com/dioad/net/ratelimit"
	"github.com/rs/zerolog"
)

// Example demonstrates network-level rate limiting by source IP.
func Example() {
	logger := zerolog.New(nil)

	// Create a generic rate limiter (10 connections per second, burst of 20)
	rl := ratelimit.NewRateLimiter(10.0, 20, logger)

	// Create a listener
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Printf("Error creating listener: %v\n", err)
		return
	}
	defer ln.Close()

	// Wrap an existing listener with rate limiting (by source IP)
	rlListener := ratelimit.NewListener(ln, rl, logger)

	fmt.Printf("Rate-limited listener created: %s\n", rlListener.Addr().Network())
	// Output: Rate-limited listener created: tcp
}
