package tls

import (
	"crypto/tls"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"

	"github.com/dioad/generics"
)

type AutoCertConfig struct {
	CacheDirectory string   `mapstructure:"cache-directory" json:",omitempty"`
	Email          string   `mapstructure:"email" json:",omitempty"`
	AllowedHosts   []string `mapstructure:"allowed-hosts" json:",omitempty"`
	DirectoryURL   string   `mapstructure:"directory-url" json:",omitempty"`
}

func NewAutocertTLSConfigFunc(c AutoCertConfig) ConfigFunc {
	return func() (*tls.Config, error) { return NewAutocertTLSConfig(c) }
}

func NewAutocertTLSConfig(c AutoCertConfig) (*tls.Config, error) {
	autoCertManager := NewAutocertManagerFromConfig(c)
	if autoCertManager == nil {
		return nil, nil
	}
	return autoCertManager.TLSConfig(), nil
}

func NewAutocertManagerFromConfig(c AutoCertConfig) *autocert.Manager {
	if generics.IsZeroValue(c) {
		return nil
	}
	autocertClient := &acme.Client{
		DirectoryURL: acme.LetsEncryptURL,
	}

	if c.DirectoryURL != "" {
		autocertClient.DirectoryURL = c.DirectoryURL
	}

	autoCertManager := autocert.Manager{
		Client:     autocertClient,
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache(c.CacheDirectory),
		HostPolicy: autocert.HostWhitelist(c.AllowedHosts...),
		Email:      c.Email,
	}

	return &autoCertManager
}
