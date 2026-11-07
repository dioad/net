package tls

import (
	"context"
	"crypto/tls"
	"net/http"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"

	"github.com/dioad/generics"
)

// newAutocertTLSConfig creates a TLS configuration with automatic
// certificate management via the tls-alpn-01 ACME challenge. Why the CA
// rejects a request or a validation is logged with ctx's logger.
func newAutocertTLSConfig(ctx context.Context, c ACMEConfig) (*tls.Config, error) {
	autoCertManager := newAutocertManager(ctx, c)
	if autoCertManager == nil {
		//nolint:nilnil // package convention: a zero-value config means "nothing to build", not an error
		return nil, nil
	}

	return autoCertManager.TLSConfig(), nil
}

// NewAutocertManagerFromConfig creates an ACME autocert manager from the
// given config. Why the CA rejects a request or a validation is logged with
// zerolog's default context logger.
func NewAutocertManagerFromConfig(c ACMEConfig) *autocert.Manager {
	return newAutocertManager(context.Background(), c)
}

func newAutocertManager(ctx context.Context, c ACMEConfig) *autocert.Manager {
	if generics.IsZeroValue(c) {
		return nil
	}
	autocertClient := &acme.Client{
		DirectoryURL: acme.LetsEncryptURL,
		HTTPClient:   &http.Client{Transport: newACMEProblemLogger(ctx, http.DefaultTransport)},
	}

	if c.DirectoryURL != "" {
		autocertClient.DirectoryURL = c.DirectoryURL
	}

	autoCertManager := autocert.Manager{
		Client:     autocertClient,
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache(c.CacheDirectory),
		HostPolicy: autocert.HostWhitelist(c.Domains...),
		Email:      c.Email,
	}

	return &autoCertManager
}
