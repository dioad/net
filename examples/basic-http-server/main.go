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
	"github.com/dioad/net/http/auth/basic"
)

func main() {
	// Create a basic auth map
	authMap := basic.AuthMap{}
	authMap.AddUserWithPlainPassword("user1", "password1")

	// Create auth handler
	authHandler, err := basic.NewHandlerWithMap(authMap)
	if err != nil {
		log.Fatalf("Error creating auth handler: %v\n", err)
	}

	// Create a simple handler
	myHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, authenticated user!\n")
	})

	// Create server with basic auth middleware
	config := diohttp.Config{ListenAddress: ":8080"}
	server := diohttp.NewServer(config)
	server.AddHandler("/protected", authHandler.Wrap(myHandler))

	// Create listener
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("Error creating listener: %v\n", err)
	}
	defer ln.Close()

	fmt.Println("Starting HTTP server with basic authentication on :8080")
	fmt.Println("Try: curl -u user1:password1 http://localhost:8080/protected")

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
