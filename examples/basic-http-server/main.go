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
)

func main() {
	// Create a simple handler
	myHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from dioad/net!\n")
	})

	// Create server
	config := diohttp.Config{ListenAddress: ":8080"}
	server := diohttp.NewServer(config)
	server.AddHandler("/hello", myHandler)

	// Create listener
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("Error creating listener: %v\n", err)
	}
	defer ln.Close()

	fmt.Println("Starting HTTP server on :8080")
	fmt.Println("Try: curl http://localhost:8080/hello")

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
