package tls

import (
	"crypto/tls"
	"reflect"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

var (
	EmptyAutoCertConfig = AutoCertConfig{}
)

type AutoCertConfig struct {
	CacheDirectory string   `mapstructure:"cache-directory" json:",omitempty"`
	Email          string   `mapstructure:"email" json:",omitempty"`
	AllowedHosts   []string `mapstructure:"allowed-hosts" json:",omitempty"`
	DirectoryURL   string   `mapstructure:"directory-url" json:",omitempty"`
}

func (c AutoCertConfig) IsEmpty() bool {
	return reflect.DeepEqual(c, EmptyAutoCertConfig)
}

func NewAutocertTLSConfigFunc(c AutoCertConfig) ConfigFunc {
	return func() (*tls.Config, error) { return NewAutocertTLSConfig(c) }
}

func NewAutocertTLSConfig(c AutoCertConfig) (*tls.Config, error) {
	autoCertManager := NewAutocertManagerFromConfig(c)
	return autoCertManager.TLSConfig(), nil
}

func NewAutocertManagerFromConfig(c AutoCertConfig) *autocert.Manager {
	autocertClient := &acme.Client{
		DirectoryURL: acme.LetsEncryptURL,
	}

	if c.DirectoryURL != "" {
		autocertClient.DirectoryURL = c.DirectoryURL
	}

	autoCertManager := autocert.Manager{
		Client: autocertClient,
		Prompt: autocert.AcceptTOS,
		Cache:  autocert.DirCache(c.CacheDirectory),
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
		HostPolicy: autocert.HostWhitelist(c.AllowedHosts...),

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
		Email: c.Email,

		// ExtraExtensions are used when generating a new CSR (Certificate Request),
		// thus allowing customization of the resulting certificate.
		// For instance, TLS Feature Extension (RFC 7633) can be used
		// to prevent an OCSP downgrade
	}

	return &autoCertManager
}
