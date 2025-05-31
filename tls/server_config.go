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

type SANConfig struct {
	DNSNames    []string `mapstructure:"dns-names" json:"dns_names,omitempty"`
	IPAddresses []string `mapstructure:"ip-addresses" json:"ip_addresses,omitempty"`
}

type CertificateSubject struct {
	Country            []string `mapstructure:"c" json:"country,omitempty"`
	Organization       []string `mapstructure:"o" json:"organization,omitempty"`
	OrganizationalUnit []string `mapstructure:"ou" json:"organizational_unit,omitempty"`
	Locality           []string `mapstructure:"l" json:"locality,omitempty"`
	Province           []string `mapstructure:"st" json:"province,omitempty"`
	StreetAddress      []string `mapstructure:"street" json:"street_address,omitempty"`
	PostalCode         []string `mapstructure:"postalcode" json:"postal_code,omitempty"`
	SerialNumber       string   `mapstructure:"serialnumber" json:"serial_number,omitempty"`
	CommonName         string   `mapstructure:"cn" json:"common_name,omitempty"`
}

type SelfSignedConfig struct {
	Subject        CertificateSubject `mapstructure:"subject" json:"subject,omitempty"`
	SANConfig      SANConfig          `mapstructure:"san-config" json:"san_config,omitempty"`
	Duration       string             `mapstructure:"duration" json:"duration,omitempty"`
	IsCA           bool               `mapstructure:"ca" json:"is_ca,omitempty"`
	Bits           int                `mapstructure:"bits" json:"bits,omitempty"`
	CacheDirectory string             `mapstructure:"cache-directory" json:"cache_directory,omitempty"`
	Alias          string             `mapstructure:"alias" json:"alias,omitempty"`
}

type LocalConfig struct {
	SinglePEMFile  string         `mapstructure:"single-pem-file" json:",omitempty"`
	Certificate    string         `mapstructure:"cert" json:",omitempty"`
	Key            string         `mapstructure:"key" json:",omitempty"`
	FileWaitConfig FileWaitConfig `mapstructure:"file-wait-config,squash" json:",omitempty,squash"`
}

type FileWaitConfig struct {
	WaitInterval uint `mapstructure:"file-wait-interval" json:",omitempty"`
	WaitMax      uint `mapstructure:"file-wait-max" json:",omitempty"`
}

type ServerConfig struct {
	ServerName string `mapstructure:"server-name"`

	AutoCertConfig AutoCertConfig `mapstructure:"auto-cert-config" json:",omitempty"`
	// EnableAutoCertManager       bool     `mapstructure:"enable-auto-cert-manager" json:",omitempty"`
	// AutoCertManagerAllowedHosts []string `mapstructure:"" json:",omitempty"`

	SelfSignedConfig SelfSignedConfig `mapstructure:"self-signed-config" json:",omitempty"`

	LocalConfig LocalConfig `mapstructure:"local" json:",omitempty"`

	ClientAuthType string `mapstructure:"client-auth-type" json:",omitempty"`
	ClientCAFile   string `mapstructure:"client-ca-file" json:",omitempty"`

	NextProtos    []string `mapstructure:"next-protos"`
	TLSMinVersion string   `mapstructure:"tls-min-version"`
}

type ConfigFunc func() (*tls.Config, error)

func configFuncFromConfig(ctx context.Context, c ServerConfig) ConfigFunc {
	if !generics.IsZeroValue(c.AutoCertConfig) {
		return NewAutocertTLSConfigFunc(c.AutoCertConfig)
	} else if !generics.IsZeroValue(c.SelfSignedConfig) {
		return NewSelfSignedTLSConfigFunc(c.SelfSignedConfig)
	} else if !generics.IsZeroValue(c.LocalConfig) {
		return NewLocalTLSConfigFunc(ctx, c.LocalConfig)
	}
	return nil
}

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

func NewLocalTLSConfigFunc(ctx context.Context, c LocalConfig) ConfigFunc {
	return func() (*tls.Config, error) { return NewLocalTLSConfig(ctx, c) }
}

func NewLocalTLSConfig(ctx context.Context, config LocalConfig) (*tls.Config, error) {
	if generics.IsZeroValue(config) {
		return nil, nil
	}
	if config.SinglePEMFile != "" {
		certs, err := CertificatesFromSinglePEMFile(ctx, config.SinglePEMFile, config.FileWaitConfig)
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
		config.FileWaitConfig)
	if err != nil {
		return nil, fmt.Errorf("error loading key pair and certs from files: %w", err)
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: cert,
	}, nil
}

func NewSelfSignedTLSConfigFunc(c SelfSignedConfig) ConfigFunc {
	return func() (*tls.Config, error) { return NewSelfSignedTLSConfig(c) }
}

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
