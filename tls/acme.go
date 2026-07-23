package tls

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/go-acme/lego/v5/challenge"

	"github.com/dioad/generics"
)

// ACME challenge types accepted by ACMEConfig.Type.
const (
	ACMEChallengeTLSALPN01 = "tls-alpn-01"
	ACMEChallengeDNS01     = "dns-01"
)

// ACMEConfig specifies parameters for obtaining a certificate via ACME,
// using either the tls-alpn-01 challenge (delegated to
// golang.org/x/crypto/acme/autocert) or the dns-01 challenge (a hand-rolled
// github.com/go-acme/lego/v5 client). The two challenge types share the same
// account identity and cache directory, but obtain certificates through
// entirely independent runtime implementations - only the config surface is
// unified here.
type ACMEConfig struct {
	// Type selects the ACME challenge mechanism: ACMEChallengeTLSALPN01 or
	// ACMEChallengeDNS01. An absent/empty Type (the zero value) defaults to
	// ACMEChallengeTLSALPN01, so callers who only need TLS-ALPN-01 can omit
	// this field entirely.
	Type string `mapstructure:"type" json:",omitempty"`

	Email          string `mapstructure:"email" json:",omitempty"`
	DirectoryURL   string `mapstructure:"directory-url" json:",omitempty"`
	CacheDirectory string `mapstructure:"cache-directory" json:",omitempty"`

	// Domains is, for Type=dns-01, the SANs to include on the single
	// certificate obtained upfront (may include wildcards, e.g.
	// "*.example.com"); for Type=tls-alpn-01 (default), the whitelist of
	// hosts autocert will provision certificates for on demand via
	// autocert.HostWhitelist. Wildcard entries are rejected for
	// tls-alpn-01 - TLS-ALPN-01 cannot validate wildcard identifiers (RFC
	// 8555 permits wildcards only via dns-01), and autocert's HostWhitelist
	// silently ignores such entries rather than erroring, so
	// NewACMETLSConfig validates this upfront instead.
	Domains []string `mapstructure:"domains" json:",omitempty"`

	// DNS01 holds fields that apply only when Type is ACMEChallengeDNS01.
	DNS01 DNS01Options `mapstructure:"dns01,squash" json:",omitzero"`
}

// DNS01Options specifies dns-01-only ACME parameters.
type DNS01Options struct {
	// PropagationTimeout bounds how long lego waits for DNS propagation of
	// the challenge TXT record. Zero uses lego's default (60s).
	PropagationTimeout time.Duration `mapstructure:"dns01-propagation-timeout" json:",omitempty"`

	// PollingInterval controls how often lego polls while waiting for
	// propagation. Zero uses lego's default (2s).
	PollingInterval time.Duration `mapstructure:"dns01-polling-interval" json:",omitempty"`

	// Provider performs Present/CleanUp of the ACME dns-01 challenge TXT
	// record. Must be set programmatically; it is not config-decodable.
	Provider challenge.Provider `mapstructure:"-" json:"-"`
}

// NewACMETLSConfigFunc creates a ConfigFunc for ACME certificate
// configuration. ctx bounds the lifetime of any background renewal
// goroutine started for the dns-01 challenge type, and is also passed to
// every ACME call made on this arm's behalf.
func NewACMETLSConfigFunc(ctx context.Context, c ACMEConfig) ConfigFunc {
	return func() (*tls.Config, error) { return NewACMETLSConfig(ctx, c) }
}

// NewACMETLSConfig obtains a certificate via ACME using the challenge type
// selected by c.Type, dispatching to the tls-alpn-01 (autocert-backed) or
// dns-01 (lego-backed) implementation.
func NewACMETLSConfig(ctx context.Context, c ACMEConfig) (*tls.Config, error) {
	if generics.IsZeroValue(c) {
		return nil, nil
	}

	switch c.Type {
	case "", ACMEChallengeTLSALPN01:
		if err := validateNoWildcards(c.Domains); err != nil {
			return nil, fmt.Errorf("acme: %w", err)
		}
		return newAutocertTLSConfig(c)
	case ACMEChallengeDNS01:
		return newDNS01TLSConfig(ctx, c)
	default:
		return nil, fmt.Errorf("acme: unrecognized challenge type %q", c.Type)
	}
}

// validateNoWildcards returns an error if any domain is a wildcard entry.
// tls-alpn-01 cannot validate wildcard identifiers (RFC 8555 permits
// wildcards only via dns-01), and autocert.HostWhitelist silently ignores
// wildcard entries rather than rejecting them, so this turns what would
// otherwise be a silent handshake-time failure into a config-time error.
func validateNoWildcards(domains []string) error {
	for _, d := range domains {
		if strings.Contains(d, "*") {
			return fmt.Errorf("wildcard domain %q is not valid for %s", d, ACMEChallengeTLSALPN01)
		}
	}
	return nil
}
