// Package main demonstrates rate limiting an HTTP server with static,
// per-principal limits.
package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	diohttp "github.com/dioad/net/http"
)

const (
	listenAddress   = ":8080"
	shutdownTimeout = 10 * time.Second
)

func main() {
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("service", "rate-limiting-http").Logger()

	if err := run(logger); err != nil {
		logger.Fatal().Err(err).Msg("server failed")
	}
}

// run serves until the process is interrupted or the server fails.
func run(logger zerolog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// One request per second with a burst of five. For the demo every
	// request belongs to the principal "user1".
	limiter := diohttp.NewRateLimiter(
		diohttp.WithStaticRateLimit(1.0, 5),
		diohttp.WithPrincipalFunc(diohttp.StaticPrincipalFunc("user1")),
		diohttp.WithRateLimitLogger(logger),
	)

	server := diohttp.NewServer(diohttp.Config{ListenAddress: listenAddress}, diohttp.WithLogger(logger))
	server.AddHandler("GET /limited", limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "Request processed successfully\n")
	})))

	if err := server.Start(ctx); err != nil {
		return err
	}
	logger.Info().Msg("try: curl http://localhost:8080/limited, several times quickly")

	select {
	case <-ctx.Done():
	case err := <-server.Failed():
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}
