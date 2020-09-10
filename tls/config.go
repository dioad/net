package tls

import (
	"crypto/tls"
	// "golang.org/x/crypto/acme/autocert"
)

type TLSConfig struct {
	TLSServerName string `mapstructure:"tls-server-name"`

	TLSServerCertificate  string `mapstructure:"tls-server-cert"`
	TLSServerKey          string `mapstructure:"tls-server-key"`
	TLSServerCAFile       string `mapstructure:"tls-server-ca-file"`
	TLSServerClientAuth   string `mapstructure:"tls-server-client-auth"`
	TLSServerClientCAFile string `mapstructure:"tls-server-client-ca-file"`

	TLSClientCertificate string `mapstructure:"tls-client-cert"`
	TLSClientKey         string `mapstructure:"tls-client-key"`
	TLSClientSkipVerify  bool   `mapstructure:"tls-client-skip-verify"`
}

func ConvertConfig(c TLSConfig) (*tls.Config, error) {
	var tlsConfig = &tls.Config{}

	//if autocertManager != nil {
	//	tlsConfig.GetCertificate = autocertManager.GetCertificate
	if c.TLSServerCertificate != "" {
		serverCertificate, err := LoadKeyPairFromFiles(c.TLSServerCertificate, c.TLSServerKey)

		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{*serverCertificate}
	}

	return tlsConfig, nil
}
