// Package main demonstrates a minimal HTTP server built with dioad/net/http.
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

	// Start returns once the listener is bound, so a port that is already in
	// use is an error here, not a log line from a goroutine.
	if err := server.Start(ctx); err != nil {
		return err
	}
	logger.Info().Msg("try: curl http://localhost:8080/hello")

	select {
	case <-ctx.Done():
	case err := <-server.Failed():
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}
