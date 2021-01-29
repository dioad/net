package tls

import (
	"crypto/tls"
	"fmt"
	"reflect"

	//"crypto/x509/pkix"
	"golang.org/x/crypto/acme/autocert"

	//"time"

	// "golang.org/x/crypto/acme/autocert"

	"github.com/pkg/errors"
)

type AutoCertConfig struct {
	CacheDirectory string   `mapstructure:"cache-directory" json:",omitempty"`
	Email          string   `mapstructure:"email" json:",omitempty"`
	AllowedHosts   []string `mapstructure:"allowed-hosts" json:",omitempty"`
}

type ServerConfig struct {
	ServerName string `mapstructure:"server-name"`

	AutoCertConfig AutoCertConfig `mapstructure:"auto-cert-config"`
	// EnableAutoCertManager       bool     `mapstructure:"enable-auto-cert-manager" json:",omitempty"`
	// AutoCertManagerAllowedHosts []string `mapstructure:"" json:",omitempty"`

	Certificate string `mapstructure:"cert" json:",omitempty"`
	Key         string `mapstructure:"key" json:",omitempty"`

	ClientAuthType string `mapstructure:"client-auth-type" json:",omitempty"`
	ClientCAFile   string `mapstructure:"client-ca-file" json:",omitempty"`
}

type ClientConfig struct {
	RootCAFile         string `mapstructure:"root-ca-file" json:",omitempty"`
	Certificate        string `mapstructure:"cert" json:",omitempty"`
	Key                string `mapstructure:"key" json:",omitempty"`
	InsecureSkipVerify bool   `mapstructure:"insecure-skip-verify"`
}

var (
	EmptyClientConfig   = ClientConfig{}
	EmptyServerConfig   = ServerConfig{}
	EmptyAutoCertConfig = AutoCertConfig{}
)

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
	if reflect.DeepEqual(c, EmptyServerConfig) {
		return nil, nil
	}

	var tlsConfig = &tls.Config{}

	if c.ServerName != "" {
		tlsConfig.ServerName = c.ServerName
	}

	if c.Certificate != "" {
		serverCertificate, err := tls.LoadX509KeyPair(c.Certificate, c.Key)

		if err != nil {
			return nil, errors.Wrap(err, "error reading server certificates")
		}
		tlsConfig.Certificates = []tls.Certificate{serverCertificate}
	} else {
		if ! reflect.DeepEqual(c.AutoCertConfig, EmptyAutoCertConfig) {
			autoCertManager := autocert.Manager{
				Prompt: autocert.AcceptTOS,
				Cache:  autocert.DirCache(c.AutoCertConfig.CacheDirectory),
				// Cache optionally stores and retrieves previously-obtained certificates
				// and other state. If nil, certs will only be cached for the lifetime of
				// the Manager. Multiple Managers can share the same Cache.
				//
				// Using a persistent Cache, such as DirCache, is strongly recommended.
				// Cache Cache

				// HostPolicy controls which domains the Manager will attempt
				// to retrieve new certificates for. It does not affect cached certs.
				//
				// If non-nil, HostPolicy is called before requesting a new cert.
				// If nil, all hosts are currently allowed. This is not recommended,
				// as it opens a potential attack where clients connect to a server
				// by IP address and pretend to be asking for an incorrect host name.
				// Manager will attempt to obtain a certificate for that host, incorrectly,
				// eventually reaching the CA's rate limit for certificate requests
				// and making it impossible to obtain actual certificates.
				//
				// See GetCertificate for more details.
				HostPolicy: autocert.HostWhitelist(c.AutoCertConfig.AllowedHosts...),

				// RenewBefore optionally specifies how early certificates should
				// be renewed before they expire.
				//
				// If zero, they're renewed 30 days before expiration.
				// RenewBefore time.Duration

				// Email optionally specifies a contact email address.
				// This is used by CAs, such as Let's Encrypt, to notify about problems
				// with issued certificates.
				//
				// If the Client's account key is already registered, Email is not used.
				Email: c.AutoCertConfig.Email,

				// ExtraExtensions are used when generating a new CSR (Certificate Request),
				// thus allowing customization of the resulting certificate.
				// For instance, TLS Feature Extension (RFC 7633) can be used
				// to prevent an OCSP downgrade attack.
				//
				// The field value is passed to crypto/x509.CreateCertificateRequest
				// in the template's ExtraExtensions field as is.
				//ExtraExtensions, []pkix.Extension
				// contains filtered or unexported fields
			}
			tlsConfig.GetCertificate = autoCertManager.GetCertificate
		}
	}

	if c.ClientCAFile != "" {
		tlsConfig.ClientAuth = convertClientAuthType(c.ClientAuthType)

		clientCAs, err := LoadCertPoolFromFile(c.ClientCAFile)
		if err != nil {
			return nil, errors.Wrap(err, "error reading client CAs")
		}
		tlsConfig.ClientCAs = clientCAs
	}

	return tlsConfig, nil
}

func ConvertClientConfig(c ClientConfig) (*tls.Config, error) {
	if c == EmptyClientConfig {
		return nil, nil
	}

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
