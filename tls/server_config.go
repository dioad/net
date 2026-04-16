package tls

import (
	"context"
	"crypto/tls"
	"fmt"
	"path/filepath"
	"time"

	"github.com/dioad/generics"

	"github.com/dioad/util"
)

// SANConfig specifies Subject Alternative Names for a certificate (DNS names and IP addresses).
type SANConfig struct {
	DNSNames    []string `mapstructure:"dns-names" json:"dns_names,omitzero"`
	IPAddresses []string `mapstructure:"ip-addresses" json:"ip_addresses,omitzero"`
}

// CertificateSubject defines X.509 certificate subject information.
type CertificateSubject struct {
	Country            []string `mapstructure:"c" json:"country,omitzero"`
	Organization       []string `mapstructure:"o" json:"organization,omitzero"`
	OrganizationalUnit []string `mapstructure:"ou" json:"organizational_unit,omitzero"`
	Locality           []string `mapstructure:"l" json:"locality,omitzero"`
	Province           []string `mapstructure:"st" json:"province,omitzero"`
	StreetAddress      []string `mapstructure:"street" json:"street_address,omitzero"`
	PostalCode         []string `mapstructure:"postalcode" json:"postal_code,omitzero"`
	SerialNumber       string   `mapstructure:"serialnumber" json:"serial_number,omitzero"`
	CommonName         string   `mapstructure:"cn" json:"common_name,omitzero"`
}

// SelfSignedConfig specifies parameters for generating a self-signed certificate.
type SelfSignedConfig struct {
	Subject        CertificateSubject `mapstructure:"subject" json:"subject"`
	SAN            SANConfig          `mapstructure:"san" json:"san"`
	Duration       string             `mapstructure:"duration" json:"duration,omitzero"`
	IsCA           bool               `mapstructure:"ca" json:"is_ca,omitzero"`
	Bits           int                `mapstructure:"bits" json:"bits,omitzero"`
	CacheDirectory string             `mapstructure:"cache-directory" json:"cache_directory,omitzero"`
	Alias          string             `mapstructure:"alias" json:"alias,omitzero"`
}

// LocalConfig specifies local certificate and key file locations.
type LocalConfig struct {
	SinglePEMFile string         `mapstructure:"single-pem-file" json:",omitzero"`
	Certificate   string         `mapstructure:"cert" json:",omitzero"`
	Key           string         `mapstructure:"key" json:",omitzero"`
	FileWait      FileWaitConfig `mapstructure:"file-wait,squash" json:",squash"`
}

// FileWaitConfig specifies wait parameters for loading certificate files.
type FileWaitConfig struct {
	WaitInterval uint `mapstructure:"file-wait-interval" json:",omitzero"`
	WaitMax      uint `mapstructure:"file-wait-max" json:",omitzero"`
}

// ServerConfig specifies TLS configuration for a server.
type ServerConfig struct {
	ServerName string `json:"server_name,omitzero" mapstructure:"server-name"`

	AutoCert AutoCertConfig `json:"auto_cert" mapstructure:"auto-cert"`
	// EnableAutoCertManager       bool     `mapstructure:"enable-auto-cert-manager" json:",omitzero"`
	// AutoCertManagerAllowedHosts []string `mapstructure:"" json:",omitzero"`

	SelfSigned SelfSignedConfig `json:"self_signed" mapstructure:"self-signed"`

	LocalConfig LocalConfig `json:"local" mapstructure:"local"`

	ClientAuthType string `mapstructure:"client-auth-type" json:"client_auth_type,omitzero"`
	ClientCAFile   string `mapstructure:"client-ca-file" json:"client_ca_file,omitzero"`

	NextProtos    []string `json:"next_protos,omitzero" mapstructure:"next-protos"`
	TLSMinVersion string   `json:"tls_min_version,omitzero" mapstructure:"tls-min-version"`
}

// ConfigFunc is a function type that returns a TLS configuration.
type ConfigFunc func() (*tls.Config, error)

func configFuncFromConfig(ctx context.Context, c ServerConfig) ConfigFunc {
	if !generics.IsZeroValue(c.AutoCert) {
		return NewAutocertTLSConfigFunc(c.AutoCert)
	} else if !generics.IsZeroValue(c.SelfSigned) {
		return NewSelfSignedTLSConfigFunc(c.SelfSigned)
	} else if !generics.IsZeroValue(c.LocalConfig) {
		return NewLocalTLSConfigFunc(ctx, c.LocalConfig)
	}
	return nil
}

// NewServerTLSConfig creates a TLS configuration for a server from the given config.
func NewServerTLSConfig(ctx context.Context, c ServerConfig) (*tls.Config, error) {
	configFunc := configFuncFromConfig(ctx, c)
	if configFunc == nil {
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
		tlsConfig.NextProtos = append(tlsConfig.NextProtos, defaultNextProtos...)
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
		return nil, fmt.Errorf("both certificate and key need to be specified")
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

	certPath := filepath.Join(cacheDirectory, fmt.Sprintf("%s.pem", alias))
	keyPath := filepath.Join(cacheDirectory, fmt.Sprintf("%s.key", alias))

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
	interval := time.Duration(waitConfig.WaitInterval) * time.Second

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

	interval := time.Duration(waitConfig.WaitInterval) * time.Second

	serverCertificate, err := util.WaitForReturn(ctx, interval, waitConfig.WaitMax, func() (*tls.Certificate, error) {
		certificate, err := tls.LoadX509KeyPair(cert, key)
		return &certificate, err
	})

	if err != nil {
		return nil, fmt.Errorf("error reading server certificates: %w", err)
	}
	return []tls.Certificate{*serverCertificate}, nil
}
