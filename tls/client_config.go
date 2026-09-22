package tls

import (
	"crypto/tls"
	"errors"
	"fmt"

	"github.com/dioad/generics"
)

// ClientConfig specifies TLS client configuration.
type ClientConfig struct {
	RootCAFile         string `json:"root_ca_file,omitempty"         mapstructure:"root-ca-file"`
	Certificate        string `json:"certificate,omitempty"          mapstructure:"cert"`
	Key                string `json:"key,omitempty"                  mapstructure:"key"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify,omitempty" mapstructure:"insecure-skip-verify"`
}

// NewClientTLSConfig creates a TLS configuration for a client from the given config.
func NewClientTLSConfig(c ClientConfig) (*tls.Config, error) {
	if generics.IsZeroValue(c) {
		//nolint:nilnil // package convention: a zero-value config means "nothing to build", not an error
		return nil, nil
	}

	cert, err := loadClientCertificate(c)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: c.InsecureSkipVerify,
	}
	if cert != nil {
		tlsConfig.Certificates = []tls.Certificate{*cert}
	}

	if c.RootCAFile != "" {
		rootCAs, err := LoadCertPoolFromFile(c.RootCAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load root CA file: %w", err)
		}
		tlsConfig.RootCAs = rootCAs
	}

	return tlsConfig, nil
}

// loadClientCertificate validates that Certificate and Key are both set or
// both empty, then loads the key pair if present. Returns (nil, nil) when
// neither is set.
func loadClientCertificate(c ClientConfig) (*tls.Certificate, error) {
	if (c.Certificate != "" && c.Key == "") || (c.Certificate == "" && c.Key != "") {
		return nil, errors.New("both certificate and key need to be specified")
	}

	if c.Certificate == "" {
		return nil, nil
	}

	cert, err := tls.LoadX509KeyPair(c.Certificate, c.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to load x509 key pair: %w", err)
	}

	return &cert, nil
}
