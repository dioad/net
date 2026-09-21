package tls

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/rs/zerolog"

	"github.com/dioad/generics"

	"github.com/dioad/util"
)

// SANConfig specifies Subject Alternative Names for a certificate (DNS names and IP addresses).
type SANConfig struct {
	DNSNames    []string `json:"dns_names,omitzero"    mapstructure:"dns-names"`
	IPAddresses []string `json:"ip_addresses,omitzero" mapstructure:"ip-addresses"`
}

// CertificateSubject defines X.509 certificate subject information.
type CertificateSubject struct {
	Country            []string `json:"country,omitzero"             mapstructure:"c"`
	Organization       []string `json:"organization,omitzero"        mapstructure:"o"`
	OrganizationalUnit []string `json:"organizational_unit,omitzero" mapstructure:"ou"`
	Locality           []string `json:"locality,omitzero"            mapstructure:"l"`
	Province           []string `json:"province,omitzero"            mapstructure:"st"`
	StreetAddress      []string `json:"street_address,omitzero"      mapstructure:"street"`
	PostalCode         []string `json:"postal_code,omitzero"         mapstructure:"postalcode"`
	SerialNumber       string   `json:"serial_number,omitzero"       mapstructure:"serialnumber"`
	CommonName         string   `json:"common_name,omitzero"         mapstructure:"cn"`
}

// SelfSignedConfig specifies parameters for generating a self-signed certificate.
type SelfSignedConfig struct {
	Subject        CertificateSubject `json:"subject"                  mapstructure:"subject"`
	SAN            SANConfig          `json:"san"                      mapstructure:"san"`
	Duration       string             `json:"duration,omitzero"        mapstructure:"duration"`
	IsCA           bool               `json:"is_ca,omitzero"           mapstructure:"ca"`
	Bits           int                `json:"bits,omitzero"            mapstructure:"bits"`
	CacheDirectory string             `json:"cache_directory,omitzero" mapstructure:"cache-directory"`
	Alias          string             `json:"alias,omitzero"           mapstructure:"alias"`
}

// LocalConfig specifies local certificate and key file locations.
type LocalConfig struct {
	SinglePEMFile string         `json:",omitzero" mapstructure:"single-pem-file"`
	Certificate   string         `json:",omitzero" mapstructure:"cert"`
	Key           string         `json:",omitzero" mapstructure:"key"`
	FileWait      FileWaitConfig `json:",omitzero" mapstructure:"file-wait,squash"`
}

// FileWaitConfig specifies wait parameters for loading certificate files.
type FileWaitConfig struct {
	WaitInterval uint `json:",omitzero" mapstructure:"file-wait-interval"`
	WaitMax      uint `json:",omitzero" mapstructure:"file-wait-max"`
}

// ServerConfig specifies TLS configuration for a server.
type ServerConfig struct {
	ServerName string `json:"server_name,omitzero" mapstructure:"server-name"`

	ACME ACMEConfig `json:"acme" mapstructure:"acme"`

	SelfSigned SelfSignedConfig `json:"self_signed" mapstructure:"self-signed"`

	LocalConfig LocalConfig `json:"local" mapstructure:"local"`

	ClientAuthType string `json:"client_auth_type,omitzero" mapstructure:"client-auth-type"`
	ClientCAFile   string `json:"client_ca_file,omitzero"   mapstructure:"client-ca-file"`

	NextProtos    []string `json:"next_protos,omitzero"     mapstructure:"next-protos"`
	TLSMinVersion string   `json:"tls_min_version,omitzero" mapstructure:"tls-min-version"`
}

// ConfigFunc is a function type that returns a TLS configuration.
type ConfigFunc func() (*tls.Config, error)

// configFuncFromConfig selects the TLS config arm to use. Arms are checked
// in a fixed order - ACME, then SelfSigned, then LocalConfig - and the
// first non-zero-value config wins; if more than one arm is configured, a
// warning is logged naming which ones and which one won, since this is
// almost always an accidental leftover from switching config rather than
// an intentional choice.
func configFuncFromConfig(ctx context.Context, c ServerConfig) ConfigFunc {
	acmeSet := !generics.IsZeroValue(c.ACME)
	selfSignedSet := !generics.IsZeroValue(c.SelfSigned)
	localSet := !generics.IsZeroValue(c.LocalConfig)

	configuredCount := 0
	for _, set := range []bool{acmeSet, selfSignedSet, localSet} {
		if set {
			configuredCount++
		}
	}
	if configuredCount > 1 {
		zerolog.Ctx(ctx).Warn().
			Bool("acme_configured", acmeSet).
			Bool("self_signed_configured", selfSignedSet).
			Bool("local_configured", localSet).
			Msg("multiple TLS config arms are configured; only the first in precedence order (ACME, then SelfSigned, then LocalConfig) is used")
	}

	switch {
	case acmeSet:
		return NewACMETLSConfigFunc(ctx, c.ACME)
	case selfSignedSet:
		return NewSelfSignedTLSConfigFunc(c.SelfSigned)
	case localSet:
		return NewLocalTLSConfigFunc(ctx, c.LocalConfig)
	}

	return nil
}

// NewServerTLSConfig creates a TLS configuration for a server from the given config.
func NewServerTLSConfig(ctx context.Context, c ServerConfig) (*tls.Config, error) {
	configFunc := configFuncFromConfig(ctx, c)
	if configFunc == nil {
		//nolint:nilnil // package convention: a zero-value config means "nothing to build", not an error
		return nil, nil
	}

	tlsConfig, err := configFunc()
	if err != nil {
		return nil, fmt.Errorf("error creating tls config: %w", err)
	}

	if c.TLSMinVersion != "" {
		tlsConfig.MinVersion = convertTLSVersion(c.TLSMinVersion)
	} else {
		tlsConfig.MinVersion = tls.VersionTLS12
	}

	if c.ServerName != "" {
		tlsConfig.ServerName = c.ServerName
	}

	defaultNextProtos := []string{"h2", "http/1.1"}
	if len(c.NextProtos) > 0 {
		defaultNextProtos = c.NextProtos
	}

	if len(tlsConfig.NextProtos) == 0 {
		tlsConfig.NextProtos = defaultNextProtos
	} else {
		for _, proto := range defaultNextProtos {
			if !slices.Contains(tlsConfig.NextProtos, proto) {
				tlsConfig.NextProtos = append(tlsConfig.NextProtos, proto)
			}
		}
	}

	if c.ClientCAFile != "" {
		tlsConfig.ClientAuth = convertClientAuthType(c.ClientAuthType)

		clientCAs, err := LoadCertPoolFromFile(c.ClientCAFile)
		if err != nil {
			return nil, fmt.Errorf("error reading client CAs: %w", err)
		}
		tlsConfig.ClientCAs = clientCAs
	}

	return tlsConfig, nil
}

// NewLocalTLSConfigFunc creates a ConfigFunc for loading certificates from local files.
func NewLocalTLSConfigFunc(ctx context.Context, c LocalConfig) ConfigFunc {
	return func() (*tls.Config, error) { return NewLocalTLSConfig(ctx, c) }
}

// NewLocalTLSConfig creates a TLS configuration from local certificate and key files.
func NewLocalTLSConfig(ctx context.Context, config LocalConfig) (*tls.Config, error) {
	if generics.IsZeroValue(config) {
		//nolint:nilnil // package convention: a zero-value config means "nothing to build", not an error
		return nil, nil
	}
	if config.SinglePEMFile != "" {
		certs, err := CertificatesFromSinglePEMFile(ctx, config.SinglePEMFile, config.FileWait)
		if err != nil {
			return nil, fmt.Errorf("error loading certificates from single pem file: %w", err)
		}

		return &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: certs,
		}, nil
	}

	if config.Certificate == "" || config.Key == "" {
		return nil, errors.New("both certificate and key need to be specified")
	}

	cert, err := CertificateFromKeyAndCertificateFiles(ctx, config.Key,
		config.Certificate,
		config.FileWait)
	if err != nil {
		return nil, fmt.Errorf("error loading key pair and certs from files: %w", err)
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: cert,
	}, nil
}

// NewSelfSignedTLSConfigFunc creates a ConfigFunc for self-signed certificate configuration.
func NewSelfSignedTLSConfigFunc(c SelfSignedConfig) ConfigFunc {
	return func() (*tls.Config, error) { return NewSelfSignedTLSConfig(c) }
}

// NewSelfSignedTLSConfig creates a TLS configuration with a self-signed certificate.
func NewSelfSignedTLSConfig(config SelfSignedConfig) (*tls.Config, error) {
	if generics.IsZeroValue(config) {
		//nolint:nilnil // package convention: a zero-value config means "nothing to build", not an error
		return nil, nil
	}

	alias := config.Alias
	if alias == "" {
		alias = "self-signed"
	}
	cacheDirectory, err := util.CreateDirPath(config.CacheDirectory, ".")
	if err != nil {
		return nil, fmt.Errorf("error creating cache directory: %w", err)
	}

	certPath := filepath.Join(cacheDirectory, alias+".pem")
	keyPath := filepath.Join(cacheDirectory, alias+".key")

	cert, _, err := CreateAndSaveSelfSignedKeyPair(config, certPath, keyPath)

	if err != nil {
		return nil, fmt.Errorf("error generating self signed certificate: %w", err)
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{*cert},
	}, nil
}

// CertificatesFromSinglePEMFile loads certificate and key from a single PEM file.
func CertificatesFromSinglePEMFile(ctx context.Context, singlePEMFile string, waitConfig FileWaitConfig) ([]tls.Certificate, error) {
	interval := time.Duration(waitConfig.WaitInterval) * time.Second // #nosec G115 -- WaitInterval is an operator-supplied config value in seconds, never near uint64/int64 boundary

	cert, err := util.WaitForReturn(ctx, interval, waitConfig.WaitMax, func() (*tls.Certificate, error) {
		return LoadKeyPairAndCertsFromFile(singlePEMFile)
	})

	if err != nil {
		return nil, fmt.Errorf("error loading key pair and certs from file: %w", err)
	}

	return []tls.Certificate{*cert}, nil
}

// CertificateFromKeyAndCertificateFiles loads certificate and key from separate files.
func CertificateFromKeyAndCertificateFiles(ctx context.Context, key, cert string, waitConfig FileWaitConfig) ([]tls.Certificate, error) {

	interval := time.Duration(waitConfig.WaitInterval) * time.Second // #nosec G115 -- WaitInterval is an operator-supplied config value in seconds, never near uint64/int64 boundary

	serverCertificate, err := util.WaitForReturn(ctx, interval, waitConfig.WaitMax, func() (*tls.Certificate, error) {
		certificate, err := tls.LoadX509KeyPair(cert, key)

		return &certificate, err
	})

	if err != nil {
		return nil, fmt.Errorf("error reading server certificates: %w", err)
	}

	return []tls.Certificate{*serverCertificate}, nil
}
