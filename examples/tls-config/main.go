// Package main demonstrates configuring an HTTPS server with a self-signed
// TLS certificate.
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/dioad/lifecycle"

	diohttp "github.com/dioad/net/http"
	diotls "github.com/dioad/net/tls"
)

const listenAddress = ":8443"

func main() {
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("service", "tls-config").Logger()

	if err := run(logger); err != nil {
		logger.Fatal().Err(err).Msg("server failed")
	}
}

// run serves until the process is interrupted or the server fails.
func run(logger zerolog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx = logger.WithContext(ctx)

	tmpDir, err := os.MkdirTemp("", "tls-example-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	_, _, err = diotls.CreateAndSaveSelfSignedKeyPair(diotls.SelfSignedConfig{
		Subject:  diotls.CertificateSubject{CommonName: "localhost"},
		SAN:      diotls.SANConfig{DNSNames: []string{"localhost"}},
		Bits:     2048,
		Duration: "24h",
	}, certFile, keyFile)
	if err != nil {
		return fmt.Errorf("failed to generate certificate: %w", err)
	}
	logger.Info().Str("certificate", certFile).Str("key", keyFile).Msg("generated self-signed certificate")

	// A server that already has a certificate on disk starts here.
	tlsConfig, err := diotls.NewServerTLSConfig(ctx, diotls.ServerConfig{
		LocalConfig: diotls.LocalConfig{Certificate: certFile, Key: keyFile},
	})
	if err != nil {
		return fmt.Errorf("failed to create TLS config: %w", err)
	}

	server := diohttp.NewServer(
		diohttp.Config{ListenAddress: listenAddress, TLSConfig: tlsConfig},
		diohttp.WithLogger(logger),
	)
	server.AddHandlerFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "Hello from secure HTTPS server!\n")
	})

	app := lifecycle.New()
	if err := app.Add("server", server); err != nil {
		return err
	}
	logger.Info().Msg("try: curl -k https://localhost:8443/ (-k because the certificate is self-signed)")

	// Run starts the server, then blocks until the process is interrupted or
	// the server fails, and stops it.
	return app.Run(ctx)
}
