// Package main demonstrates rate limiting a raw TCP listener by source IP
// using ratelimit.Listener.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/dioad/net/ratelimit"
)

const listenAddress = ":8080"

func main() {
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("service", "rate-limiting-network").Logger()

	if err := run(logger); err != nil {
		logger.Fatal().Err(err).Msg("server failed")
	}
}

// run accepts connections until the process is interrupted.
func run(logger zerolog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Ten connections per second with a burst of twenty, for each source IP.
	limiter := ratelimit.NewRateLimiterWithOptions(
		ratelimit.WithRateLimiterStaticLimits(10.0, 20),
		ratelimit.WithRateLimiterLogger(logger),
	)
	defer limiter.Stop()

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", listenAddress)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", listenAddress, err)
	}
	listener := ratelimit.NewListener(ln, limiter, logger)
	logger.Info().Str("address", ln.Addr().String()).Msg("try: nc localhost 8080")

	// Closing the listener is what ends the accept loop below.
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if errors.Is(err, net.ErrClosed) {
			logger.Info().Msg("listener stopped")

			return nil
		}
		if err != nil {
			logger.Error().Err(err).Msg("failed to accept connection")

			continue
		}

		go echo(conn, logger.With().Str("remote_addr", conn.RemoteAddr().String()).Logger())
	}
}

// echo writes back what the client sends until it sends "quit".
func echo(conn net.Conn, logger zerolog.Logger) {
	defer func() { _ = conn.Close() }()
	logger.Info().Msg("connection accepted")

	_, _ = io.WriteString(conn, "Hello! This connection is rate-limited by source IP.\nType 'quit' to disconnect.\n\n")

	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				logger.Error().Err(err).Msg("failed to read from connection")
			}

			break
		}

		if msg := string(buf[:n]); msg == "quit\n" || msg == "quit\r\n" {
			_, _ = io.WriteString(conn, "Goodbye!\n")

			break
		}
		_, _ = conn.Write(buf[:n])
	}

	logger.Info().Msg("connection closed")
}
