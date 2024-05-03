package tls

import (
	"crypto/tls"
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/dioad/util"
)

var (
	EmptyServerConfig = ServerConfig{}
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
	CacheDirectory string             `mapstructure:"cache-directory" json:"cache_directory,omitempty"`
	Alias          string             `mapstructure:"alias" json:"alias,omitempty"`
}

func (c SelfSignedConfig) IsEmpty() bool {
	return reflect.DeepEqual(c, EmptySelfSignedConfig)
}

type LocalConfig struct {
	SinglePEMFile  string         `mapstructure:"single-pem-file" json:",omitempty"`
	Certificate    string         `mapstructure:"cert" json:",omitempty"`
	Key            string         `mapstructure:"key" json:",omitempty"`
	FileWaitConfig FileWaitConfig `mapstructure:"file-wait-config,squash" json:",omitempty,squash"`
}

func (c LocalConfig) IsEmpty() bool {
	return reflect.DeepEqual(c, EmptyLocalConfig)
}

type FileWaitConfig struct {
	WaitInterval int `mapstructure:"file-wait-interval" json:",omitempty"`
	WaitMax      int `mapstructure:"file-wait-max" json:",omitempty"`
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

func (s ServerConfig) IsEmpty() bool {
	return reflect.DeepEqual(s, EmptyServerConfig)
}

func convertTLSVersion(version string) uint16 {
	switch version {
	case "TLS13":
		return tls.VersionTLS13
	case "TLS12":
		return tls.VersionTLS12
	case "TLS11":
		return tls.VersionTLS11
	case "TLS10":
		return tls.VersionTLS10
	default:
		return tls.VersionTLS12
	}
}

type ConfigFunc func() (*tls.Config, error)

func configFuncFromConfig(c ServerConfig) ConfigFunc {
	if !c.AutoCertConfig.IsEmpty() {
		return NewAutocertTLSConfigFunc(c.AutoCertConfig)
	} else if !c.SelfSignedConfig.IsEmpty() {
		return NewSelfSignedTLSConfigFunc(c.SelfSignedConfig)
	} else if !c.LocalConfig.IsEmpty() {
		return NewLocalTLSConfigFunc(c.LocalConfig)
	}
	return nil
}

func NewServerTLSConfig(c ServerConfig) (*tls.Config, error) {
	configFunc := configFuncFromConfig(c)
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

	if len(c.NextProtos) == 0 {
		tlsConfig.NextProtos = []string{"h2", "http/1.1"}
	} else {
		tlsConfig.NextProtos = c.NextProtos
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

func NewLocalTLSConfigFunc(c LocalConfig) ConfigFunc {
	return func() (*tls.Config, error) { return NewLocalTLSConfig(c) }
}

func NewLocalTLSConfig(config LocalConfig) (*tls.Config, error) {
	if config.SinglePEMFile != "" {
		certs, err := CertificatesFromSinglePEMFile(config.SinglePEMFile, config.FileWaitConfig)
		if err != nil {
			return nil, fmt.Errorf("error loading certificates from single pem file: %w", err)
		}

		return &tls.Config{Certificates: certs}, nil
	}

	if config.Certificate == "" || config.Key == "" {
		return nil, fmt.Errorf("both certificate and key need to be specified")
	}

	cert, err := CertificateFromKeyAndCertificateFiles(config.Key,
		config.Certificate,
		config.FileWaitConfig)
	if err != nil {
		return nil, fmt.Errorf("error loading key pair and certs from files: %w", err)
	}

	return &tls.Config{Certificates: cert}, nil
}

func NewSelfSignedTLSConfigFunc(c SelfSignedConfig) ConfigFunc {
	return func() (*tls.Config, error) { return NewSelfSignedTLSConfig(c) }
}

func NewSelfSignedTLSConfig(config SelfSignedConfig) (*tls.Config, error) {
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
	return &tls.Config{Certificates: []tls.Certificate{*cert}}, nil
}

func CertificatesFromSinglePEMFile(singlePEMFile string, waitConfig FileWaitConfig) ([]tls.Certificate, error) {
	err := util.WaitForFiles(waitConfig.WaitInterval, waitConfig.WaitMax, singlePEMFile)
	if err != nil {
		return nil, fmt.Errorf("error waiting for file: %w", err)
	}
	cert, err := LoadKeyPairAndCertsFromFile(singlePEMFile)
	if err != nil {
		return nil, fmt.Errorf("error loading key pair and certs from file: %w", err)
	}
	return []tls.Certificate{*cert}, nil
}

func CertificateFromKeyAndCertificateFiles(key, cert string, waitConfig FileWaitConfig) ([]tls.Certificate, error) {
	err := util.WaitForFiles(waitConfig.WaitInterval, waitConfig.WaitMax, cert, key)
	if err != nil {
		return nil, fmt.Errorf("error waiting for files: %w", err)
	}
	serverCertificate, err := tls.LoadX509KeyPair(cert, key)

	if err != nil {
		return nil, fmt.Errorf("error reading server certificates: %w", err)
	}
	return []tls.Certificate{serverCertificate}, nil
}
