package tls

import (
	"crypto/tls"
	"fmt"
	// "golang.org/x/crypto/acme/autocert"
)

type ServerConfig struct {
	ServerName string `mapstructure:"server-name"`

	Certificate string `mapstructure:"cert" json:"omitempty"`
	Key         string `mapstructure:"key" json:"omitempty"`

	ClientAuthType string `mapstructure:"client-auth-type" json:"omitempty"`
	ClientCAFile   string `mapstructure:"client-ca-file" json:"omitempty"`
}

type ClientConfig struct {
	RootCAFile         string `mapstructure:"root-ca-file" json:"omitempty"`
	Certificate        string `mapstructure:"cert" json:"omitempty"`
	Key                string `mapstructure:"key" json:"omitempty"`
	InsecureSkipVerify bool   `mapstructure:"insecure-skip-verify"`
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

func ConvertServerConfig(c ServerConfig) (*tls.Config, error) {
	var tlsConfig = &tls.Config{}

	if c.ServerName != "" {
		tlsConfig.ServerName = c.ServerName
	}

	tlsConfig.ClientAuth = convertClientAuthType(c.ClientAuthType)

	//if autocertManager != nil {
	//	tlsConfig.GetCertificate = autocertManager.GetCertificate
	if c.Certificate != "" {
		serverCertificate, err := tls.LoadX509KeyPair(c.Certificate, c.Key)

		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{serverCertificate}
	}

	if c.ClientCAFile != "" {
		clientCAs, err := LoadCertPoolFromFile(c.ClientCAFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.ClientCAs = clientCAs
	}

	return tlsConfig, nil
}

func ConvertClientConfig(c ClientConfig) (*tls.Config, error) {
	var tlsConfig = &tls.Config{}

	if (c.Certificate != "" && c.Key == "") || (c.Certificate == "" && c.Key != "") {
		return nil, fmt.Errorf("both certificate and key need to be specified")
	}

	if c.Certificate != "" && c.Key != "" {
		clientCertificate, err := tls.LoadX509KeyPair(c.Certificate, c.Key)

		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{clientCertificate}
	}

	if c.RootCAFile != "" {
		rootCAs, err := LoadCertPoolFromFile(c.RootCAFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.RootCAs = rootCAs
	}

	tlsConfig.InsecureSkipVerify = c.InsecureSkipVerify

	return tlsConfig, nil
}
