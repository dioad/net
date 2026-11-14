// Package main demonstrates IP-based access control: authorizing individual
// requests against a network ACL and wrapping a net.Listener so that only
// connections from allowed IPs are accepted.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/dioad/net/authz"
)

func main() {
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("service", "ip-acl").Logger()

	if err := run(logger); err != nil {
		logger.Fatal().Err(err).Msg("server failed")
	}
}

// run prints the authorization checks, then accepts connections on two
// ACL-protected listeners until the process is interrupted.
func run(logger zerolog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Println("Part 1: IP authorization checks")
	if err := demonstrateIPChecks(); err != nil {
		return err
	}
	fmt.Println()
	fmt.Println("Part 2: ACL-protected listeners")
	fmt.Println("  nc localhost 9001  # allows localhost: connects")
	fmt.Println("  nc localhost 9002  # denies localhost: closed at once")
	fmt.Println()

	allowListener, err := listenWithACL(ctx, "127.0.0.1:9001", authz.NetworkACLConfig{AllowedNets: []string{"127.0.0.0/8"}}, logger)
	if err != nil {
		return err
	}
	defer func() { _ = allowListener.Close() }()

	denyListener, err := listenWithACL(ctx, "127.0.0.1:9002", authz.NetworkACLConfig{DeniedNets: []string{"127.0.0.0/8"}}, logger)
	if err != nil {
		return err
	}
	defer func() { _ = denyListener.Close() }()

	var wg sync.WaitGroup
	wg.Go(func() { acceptConnections(allowListener, "allow", logger) })
	wg.Go(func() { acceptConnections(denyListener, "deny", logger) })

	// Closing the listeners is what ends the accept loops.
	<-ctx.Done()
	_ = allowListener.Close()
	_ = denyListener.Close()
	wg.Wait()
	logger.Info().Msg("listeners stopped")

	return nil
}

// listenWithACL listens on address and wraps the listener so that only
// connections the ACL authorises are served.
func listenWithACL(ctx context.Context, address string, cfg authz.NetworkACLConfig, logger zerolog.Logger) (*authz.Listener, error) {
	acl, err := authz.NewNetworkACL(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create ACL for %s: %w", address, err)
	}

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	return authz.NewListener(ln, acl, logger), nil
}

func demonstrateIPChecks() error {
	acl, err := authz.NewNetworkACL(authz.NetworkACLConfig{
		AllowedNets: []string{"10.0.0.0/8"},
		DeniedNets:  []string{"10.0.0.5"},
	})
	if err != nil {
		return fmt.Errorf("failed to create ACL: %w", err)
	}

	fmt.Println("  allowed: 10.0.0.0/8, denied: 10.0.0.5")

	// An address inside the allowed network, the denied address, and an
	// address outside the allowed network.
	for _, addr := range []string{"10.0.1.1:12345", "10.0.0.5:12345", "192.168.1.1:12345"} {
		if err := acl.AuthoriseFromString(addr); err != nil {
			fmt.Printf("  %s: denied (%v)\n", addr, err)

			continue
		}
		fmt.Printf("  %s: allowed\n", addr)
	}

	return nil
}

func acceptConnections(listener *authz.Listener, listenerType string, logger zerolog.Logger) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			// Check if listener was closed
			if errors.Is(err, net.ErrClosed) {
				return
			}
			logger.Error().Err(err).Str("type", listenerType).Msg("accept error")

			continue
		}

		// Connection returned (authz.Listener closes denied connections but still returns them)
		// The authz.Listener itself logs "access denied" for rejected connections
		logger.Info().
			Str("type", listenerType).
			Str("remote_addr", conn.RemoteAddr().String()).
			Msg("connection received")

		// Handle the connection (it may already be closed if denied by ACL)
		go handleConnection(conn, logger)
	}
}

func handleConnection(conn net.Conn, logger zerolog.Logger) {
	defer func() { _ = conn.Close() }()

	// Send welcome message
	welcome := fmt.Sprintf("Welcome! You connected from %s\n", conn.RemoteAddr())
	welcome += "This connection was authorized by the ACL.\n"
	welcome += "Type 'quit' to disconnect.\n\n"
	_, _ = io.WriteString(conn, welcome)

	// Simple echo server
	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				logger.Error().Err(err).Msg("read error")
			}

			break
		}

		if n > 0 {
			msg := string(buf[:n])
			if msg == "quit\n" || msg == "quit\r\n" {
				_, _ = io.WriteString(conn, "Goodbye!\n")

				break
			}
			// Echo back
			_, _ = conn.Write(buf[:n])
		}
	}

	logger.Info().
		Str("remote_addr", conn.RemoteAddr().String()).
		Msg("connection closed")
}
