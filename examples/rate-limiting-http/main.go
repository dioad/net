package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	diohttp "github.com/dioad/net/http"
	"github.com/rs/zerolog"
)

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Create rate limiter (1 request per second, burst of 5)
	limiter := diohttp.NewRateLimiter(1.0, 5, logger)

	// Create a simple handler
	myHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Request processed successfully\n")
	})

	// Use as middleware for a specific principal
	handler := limiter.Middleware("user1")(myHandler)

	// Create server
	config := diohttp.Config{ListenAddress: ":8080"}
	server := diohttp.NewServer(config)
	server.AddHandler("/limited", handler)

	// Create listener
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("Error creating listener: %v\n", err)
	}
	defer ln.Close()

	fmt.Println("Starting HTTP server with rate limiting on :8080")
	fmt.Println("Rate limit: 1 request/second with burst of 5 for principal 'user1'")
	fmt.Println("Try: curl http://localhost:8080/limited")
	fmt.Println("Try multiple times quickly to see rate limiting in action")

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
