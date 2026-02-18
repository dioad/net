package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	diohttp "github.com/dioad/net/http"
)

// mySource implements RateLimitSource for dynamic rate limiting.
type mySource struct{}

func (s *mySource) GetLimit(principal string) (float64, int, bool) {
	if principal == "premium" {
		return 100.0, 100, true
	}
	return 1.0, 5, true
}

func myPrincipalFunc(r *http.Request) (string, error) {
	if r.URL.Path == "/premium" {
		return "premium", nil
	}
	return "standard", nil
}

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Create rate limiter with dynamic source
	limiter := diohttp.NewRateLimiter(
		diohttp.WithRateLimitSource(&mySource{}),
		diohttp.WithPrincipalFunc(myPrincipalFunc),
		diohttp.WithRateLimitLogger(logger),
	)

	// Create a simple handler
	myHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Request processed successfully\n")
	})

	// Use as middleware for premium users
	handler := limiter.Middleware(myHandler)

	// // Use as middleware for standard users
	// standardHandler := limiter.Middleware(myHandler)

	// Create server
	config := diohttp.Config{ListenAddress: ":8080"}
	server := diohttp.NewServer(config)
	server.AddHandler("/premium", handler)
	server.AddHandler("/standard", handler)

	// Create listener
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("Error creating listener: %v\n", err)
	}
	defer ln.Close()

	fmt.Println("Starting HTTP server with dynamic rate limiting on :8080")
	fmt.Println("Premium users (/premium): 100 requests/second with burst of 100")
	fmt.Println("Standard users (/standard): 1 request/second with burst of 5")
	fmt.Println("Try: curl http://localhost:8080/premium")
	fmt.Println("Try: curl http://localhost:8080/standard")

	// Start server in goroutine
	go func() {
		if err := server.Serve(ln); err != nil {
			log.Printf("Server error: %v\n", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down server...")
}
