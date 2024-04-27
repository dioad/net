package tls

import (
	"crypto/tls"
	"fmt"
	"reflect"
	// "time"
	// "golang.org/x/crypto/acme/autocert"
)

var (
	EmptyClientConfig = ClientConfig{}
)

type ClientConfig struct {
	RootCAFile         string `mapstructure:"root-ca-file" json:",omitempty"`
	Certificate        string `mapstructure:"cert" json:",omitempty"`
	Key                string `mapstructure:"key" json:",omitempty"`
	InsecureSkipVerify bool   `mapstructure:"insecure-skip-verify"`
}

func (c ClientConfig) IsEmpty() bool {
	return reflect.DeepEqual(c, EmptyClientConfig)
}

func convertClientAuthType(authType string) tls.ClientAuthType {
	switch authType {
	case "RequestClientCert":
		return tls.RequestClientCert
	case "RequireAnyClientCert":
		return tls.RequireAnyClientCert
	case "VerifyClientCertIfGiven":
		return tls.VerifyClientCertIfGiven
	case "RequireAndVerifyClientCert":
		return tls.RequireAndVerifyClientCert
	default:
		return tls.NoClientCert
	}
}

func NewClientTLSConfig(c ClientConfig) (*tls.Config, error) {
	if c == EmptyClientConfig {
		return nil, nil
	}

	var tlsConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if (c.Certificate != "" && c.Key == "") || (c.Certificate == "" && c.Key != "") {
		return nil, fmt.Errorf("both certificate and key need to be specified")
	}

	if c.Certificate != "" && c.Key != "" {
		clientCertificate, err := tls.LoadX509KeyPair(c.Certificate, c.Key)

		if err != nil {
			return nil, fmt.Errorf("failed to load x509 key pair: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{clientCertificate}
	}

	if c.RootCAFile != "" {
		rootCAs, err := LoadCertPoolFromFile(c.RootCAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load root CA file: %w", err)
		}
		tlsConfig.RootCAs = rootCAs
	}

	tlsConfig.InsecureSkipVerify = c.InsecureSkipVerify

	return tlsConfig, nil
}
