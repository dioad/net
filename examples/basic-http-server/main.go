// Package main demonstrates a minimal HTTP server built with dioad/net/http.
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
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("service", "basic-http-server").Logger()

	if err := run(logger); err != nil {
		logger.Fatal().Err(err).Msg("server failed")
	}
}

// run serves until the process is interrupted or the server fails.
func run(logger zerolog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// WithLogger puts the logger in each request's context, where a handler
	// finds it with zerolog.Ctx.
	server := diohttp.NewServer(diohttp.Config{ListenAddress: listenAddress}, diohttp.WithLogger(logger))
	server.AddHandlerFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		zerolog.Ctx(r.Context()).Info().Msg("saying hello")
		_, _ = io.WriteString(w, "Hello from dioad/net!\n")
	})

	app := lifecycle.New()
	if err := app.Add("server", server); err != nil {
		return err
	}
	logger.Info().Msg("try: curl http://localhost:8080/hello")

	// Run starts the server, then blocks until the process is interrupted or
	// the server fails, and stops it. A port that is already in use is an
	// error from Run, not a log line from a goroutine.
	return app.Run(ctx)
}
