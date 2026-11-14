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

	"github.com/rs/zerolog"

	"github.com/dioad/lifecycle"

	diohttp "github.com/dioad/net/http"
)

const listenAddress = ":8080"

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

	app := lifecycle.New()
	if err := app.Add("server", server); err != nil {
		return err
	}
	logger.Info().Msg("try: curl http://localhost:8080/limited, several times quickly")

	// Run starts the server, then blocks until the process is interrupted or
	// the server fails, and stops it.
	return app.Run(ctx)
}
