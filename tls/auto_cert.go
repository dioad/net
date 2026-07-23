package tls

import (
	"crypto/tls"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"

	"github.com/dioad/generics"
)

// newAutocertTLSConfig creates a TLS configuration with automatic
// certificate management via the tls-alpn-01 ACME challenge.
func newAutocertTLSConfig(c ACMEConfig) (*tls.Config, error) {
	autoCertManager := NewAutocertManagerFromConfig(c)
	if autoCertManager == nil {
		return nil, nil
	}
	return autoCertManager.TLSConfig(), nil
}

// NewAutocertManagerFromConfig creates an ACME autocert manager from the
// given config.
func NewAutocertManagerFromConfig(c ACMEConfig) *autocert.Manager {
	if generics.IsZeroValue(c) {
		return nil
	}
	autocertClient := &acme.Client{
		DirectoryURL: acme.LetsEncryptURL,
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
