package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/dioad/net/authz"
	"github.com/rs/zerolog"
)

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	fmt.Println("=== IP-based Access Control Demo ===")
	fmt.Println()

	// Demonstration 1: Simple IP authorization checks
	fmt.Println("Part 1: IP Authorization Checks")
	fmt.Println("--------------------------------")
	demonstrateIPChecks()
	fmt.Println()

	// Demonstration 2: ACL-protected listeners
	fmt.Println("Part 2: ACL-Protected Network Listeners")
	fmt.Println("---------------------------------------")

	// Create ACL that allows localhost/localnet
	allowLocalACL, err := authz.NewNetworkACL(authz.NetworkACLConfig{
		AllowedNets: []string{"127.0.0.0/8"},
	})
	if err != nil {
		log.Fatalf("Error creating allow-local ACL: %v\n", err)
	}

	// Create ACL that denies localhost/localnet
	denyLocalACL, err := authz.NewNetworkACL(authz.NetworkACLConfig{
		DeniedNets: []string{"127.0.0.0/8"},
	})
	if err != nil {
		log.Fatalf("Error creating deny-local ACL: %v\n", err)
	}

	// Create listener that ALLOWS localhost connections
	allowListener, err := net.Listen("tcp", "127.0.0.1:9001")
	if err != nil {
		log.Fatalf("Error creating allow listener: %v\n", err)
	}
	defer allowListener.Close()

	aclAllowListener := &authz.Listener{
		NetworkACL: allowLocalACL,
		Listener:   allowListener,
		Logger:     logger,
	}

	// Create listener that DENIES localhost connections
	denyListener, err := net.Listen("tcp", "127.0.0.1:9002")
	if err != nil {
		log.Fatalf("Error creating deny listener: %v\n", err)
	}
	defer denyListener.Close()

	aclDenyListener := &authz.Listener{
		NetworkACL: denyLocalACL,
		Listener:   denyListener,
		Logger:     logger,
	}

	fmt.Println("Started two ACL-protected listeners:")
	fmt.Println()
	fmt.Println("  Listener 1 (Port 9001): ALLOWS localhost (127.0.0.0/8)")
	fmt.Println("  Listener 2 (Port 9002): DENIES localhost (127.0.0.0/8)")
	fmt.Println()
	fmt.Println("Test these listeners with:")
	fmt.Println("  nc localhost 9001  # Should connect successfully")
	fmt.Println("  nc localhost 9002  # Connection will be rejected")
	fmt.Println()
	fmt.Println("Or use telnet:")
	fmt.Println("  telnet localhost 9001")
	fmt.Println("  telnet localhost 9002")
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop the servers")
	fmt.Println()

	// Handle connections in goroutines
	var wg sync.WaitGroup

	// Start accepting on allow listener
	wg.Add(1)
	go func() {
		defer wg.Done()
		acceptConnections(aclAllowListener, "ALLOW", logger)
	}()

	// Start accepting on deny listener
	wg.Add(1)
	go func() {
		defer wg.Done()
		acceptConnections(aclDenyListener, "DENY", logger)
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down listeners...")
	aclAllowListener.Close()
	aclDenyListener.Close()

	// Wait for goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("All listeners stopped")
	case <-time.After(2 * time.Second):
		fmt.Println("Timeout waiting for listeners to stop")
	}
}

func demonstrateIPChecks() {
	// Create network ACL
	acl, err := authz.NewNetworkACL(authz.NetworkACLConfig{
		AllowedNets: []string{"10.0.0.0/8"},
		DeniedNets:  []string{"10.0.0.5"},
	})
	if err != nil {
		log.Fatalf("Error creating ACL: %v\n", err)
	}

	fmt.Println("Network ACL created with:")
	fmt.Println("  Allowed: 10.0.0.0/8")
	fmt.Println("  Denied: 10.0.0.5")
	fmt.Println()

	// Check if IP is authorised
	clientIP := "10.0.1.1:12345"
	if authorised, err := acl.AuthoriseFromString(clientIP); err != nil {
		log.Printf("Error checking authorization: %v\n", err)
	} else if authorised {
		fmt.Printf("✓ %s - Access allowed\n", clientIP)
	} else {
		fmt.Printf("✗ %s - Access denied\n", clientIP)
	}

	// Check denied IP
	deniedIP := "10.0.0.5:12345"
	if authorised, err := acl.AuthoriseFromString(deniedIP); err != nil {
		log.Printf("Error checking authorization: %v\n", err)
	} else if authorised {
		fmt.Printf("✓ %s - Access allowed\n", deniedIP)
	} else {
		fmt.Printf("✗ %s - Access denied (explicitly blocked)\n", deniedIP)
	}

	// Check IP outside allowed range
	outsideIP := "192.168.1.1:12345"
	if authorised, err := acl.AuthoriseFromString(outsideIP); err != nil {
		log.Printf("Error checking authorization: %v\n", err)
	} else if authorised {
		fmt.Printf("✓ %s - Access allowed\n", outsideIP)
	} else {
		fmt.Printf("✗ %s - Access denied (outside allowed range)\n", outsideIP)
	}
}

func acceptConnections(listener *authz.Listener, listenerType string, logger zerolog.Logger) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			// Check if listener was closed
			if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
				return
			}
			logger.Error().Err(err).Str("type", listenerType).Msg("accept error")
			continue
		}

		// Connection was authorized and accepted
		if conn != nil {
			logger.Info().
				Str("type", listenerType).
				Str("remote_addr", conn.RemoteAddr().String()).
				Msg("✓ Connection ALLOWED")

			// Handle the connection
			go handleConnection(conn, logger)
		} else {
			// Connection was denied (conn is nil when ACL rejects)
			logger.Warn().
				Str("type", listenerType).
				Msg("✗ Connection DENIED by ACL")
		}
	}
}

func handleConnection(conn net.Conn, logger zerolog.Logger) {
	defer conn.Close()

	// Send welcome message
	welcome := fmt.Sprintf("Welcome! You connected from %s\n", conn.RemoteAddr())
	welcome += "This connection was authorized by the ACL.\n"
	welcome += "Type 'quit' to disconnect.\n\n"
	io.WriteString(conn, welcome)

	// Simple echo server
	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				logger.Error().Err(err).Msg("read error")
			}
			break
		}

		if n > 0 {
			msg := string(buf[:n])
			if msg == "quit\n" || msg == "quit\r\n" {
				io.WriteString(conn, "Goodbye!\n")
				break
			}
			// Echo back
			conn.Write(buf[:n])
		}
	}

	logger.Info().
		Str("remote_addr", conn.RemoteAddr().String()).
		Msg("Connection closed")
}
