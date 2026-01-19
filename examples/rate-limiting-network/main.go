package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/dioad/net/ratelimit"
	"github.com/rs/zerolog"
)

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Create a generic rate limiter (10 connections per second, burst of 20)
	rl := ratelimit.NewRateLimiter(10.0, 20, logger)

	// Create a listener
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("Error creating listener: %v\n", err)
	}
	defer ln.Close()

	// Wrap an existing listener with rate limiting (by source IP)
	rlListener := ratelimit.NewListener(ln, rl, logger)

	fmt.Println("Starting TCP server with network rate limiting on :8080")
	fmt.Println("Rate limit: 10 connections/second with burst of 20 per source IP")
	fmt.Println("Try: nc localhost 8080 (or telnet localhost 8080)")

	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutting down server...")
		rlListener.Close()
	}()

	// Accept and handle connections
	for {
		conn, err := rlListener.Accept()
		if err != nil {
			// Check if it's because we're shutting down
			if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
				break
			}
			log.Printf("Error accepting connection: %v\n", err)
			continue
		}

		// Handle connection in goroutine
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	
	fmt.Printf("New connection from %s\n", conn.RemoteAddr())
	
	// Send welcome message
	io.WriteString(conn, "Hello! You've connected to the rate-limited server.\n")
	io.WriteString(conn, "This connection is rate-limited by source IP.\n")
	io.WriteString(conn, "Type 'quit' to disconnect.\n\n")

	// Echo server
	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading from connection: %v\n", err)
			}
			break
		}
		
		if n > 0 {
			msg := string(buf[:n])
			if msg == "quit\n" || msg == "quit\r\n" {
				io.WriteString(conn, "Goodbye!\n")
				break
			}
			conn.Write(buf[:n])
		}
	}
	
	fmt.Printf("Connection from %s closed\n", conn.RemoteAddr())
}
