// Package main demonstrates rate limiting an HTTP server with limits
// supplied dynamically by a custom RateLimitSource.
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

// tierSource implements RateLimitSource: it gives each principal its own
// limit.
type tierSource struct{}

func (tierSource) GetLimit(principal string) (float64, int, bool) {
	if principal == "premium" {
		return 100.0, 100, true
	}

	return 1.0, 5, true
}

// principalFromPath names the principal a request is limited as. A real
// server would take it from the authenticated identity.
func principalFromPath(r *http.Request) (string, error) {
	if r.URL.Path == "/premium" {
		return "premium", nil
	}

	return "standard", nil
}

func main() {
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("service", "rate-limiting-dynamic").Logger()

	if err := run(logger); err != nil {
		logger.Fatal().Err(err).Msg("server failed")
	}
}

// run serves until the process is interrupted or the server fails.
func run(logger zerolog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	limiter := diohttp.NewRateLimiter(
		diohttp.WithRateLimitSource(tierSource{}),
		diohttp.WithPrincipalFunc(principalFromPath),
		diohttp.WithRateLimitLogger(logger),
	)
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "Request processed successfully\n")
	}))

	server := diohttp.NewServer(diohttp.Config{ListenAddress: listenAddress}, diohttp.WithLogger(logger))
	server.AddHandler("GET /premium", handler)
	server.AddHandler("GET /standard", handler)

	app := lifecycle.New()
	if err := app.Add("server", server); err != nil {
		return err
	}
	logger.Info().Msg("try: curl http://localhost:8080/premium (100 requests/second) and /standard (1 request/second)")

	// Run starts the server, then blocks until the process is interrupted or
	// the server fails, and stops it.
	return app.Run(ctx)
}
