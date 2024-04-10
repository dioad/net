package tls

import (
	"crypto/tls"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"golang.org/x/crypto/acme"
	//"crypto/x509/pkix"
	"golang.org/x/crypto/acme/autocert"

	//"time"

	// "golang.org/x/crypto/acme/autocert"

	"github.com/pkg/errors"
)

var (
	EmptyClientConfig   = ClientConfig{}
	EmptyServerConfig   = ServerConfig{}
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

type ServerConfig struct {
	ServerName string `mapstructure:"server-name"`

	AutoCertConfig AutoCertConfig `mapstructure:"auto-cert-config"`
	// EnableAutoCertManager       bool     `mapstructure:"enable-auto-cert-manager" json:",omitempty"`
	// AutoCertManagerAllowedHosts []string `mapstructure:"" json:",omitempty"`

	SinglePEMFile string `mapstructure:"single-pem-file" json:",omitempty"`
	Certificate   string `mapstructure:"cert" json:",omitempty"`
	Key           string `mapstructure:"key" json:",omitempty"`

	FileWaitInterval int `mapstructure:"file-wait-interval" json:",omitempty"`
	FileWaitMax      int `mapstructure:"file-wait-max" json:",omitempty"`

	ClientAuthType string `mapstructure:"client-auth-type" json:",omitempty"`
	ClientCAFile   string `mapstructure:"client-ca-file" json:",omitempty"`

	NextProtos []string `mapstructure:"next-protos"`
}

func (s ServerConfig) IsEmpty() bool {
	return reflect.DeepEqual(s, EmptyServerConfig)
}

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

// WaitForFile waits for a file to exist, it will check every interval seconds up until max seconds.
func waitForFiles(interval, max int, files ...string) error {
	if interval <= 0 {
		interval = 0
	}
	if max <= 0 {
		max = 1
	}
	for i := 0; i < max; i++ {
		if filesExist(files...) {
			return nil
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}
	return fmt.Errorf("one or more of %s not found", strings.Join(files, ", "))
}

func filesExist(files ...string) bool {
	for _, file := range files {
		if _, err := os.Stat(file); err != nil {
			return false
		}
	}
	return true
}

func AutocertManagerFromConfig(c AutoCertConfig) *autocert.Manager {
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

func ConvertServerConfig(c ServerConfig) (*tls.Config, error) {
	if c.IsEmpty() {
		return nil, nil
	}

	var tlsConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if c.ServerName != "" {
		tlsConfig.ServerName = c.ServerName
	}

	if len(c.NextProtos) == 0 {
		tlsConfig.NextProtos = []string{"h2", "http/1.1"}
	} else {
		tlsConfig.NextProtos = c.NextProtos
	}

	if c.SinglePEMFile != "" {
		err := waitForFiles(c.FileWaitInterval, c.FileWaitMax, c.SinglePEMFile)
		if err != nil {
			return nil, err
		}
		cert, err := LoadKeyPairAndCertsFromFile(c.SinglePEMFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{*cert}
	} else if c.Certificate != "" {
		err := waitForFiles(c.FileWaitInterval, c.FileWaitMax, c.Certificate, c.Key)
		if err != nil {
			return nil, err
		}
		serverCertificate, err := tls.LoadX509KeyPair(c.Certificate, c.Key)

		if err != nil {
			return nil, errors.Wrap(err, "error reading server certificates")
		}
		tlsConfig.Certificates = []tls.Certificate{serverCertificate}
	} else {
		if !c.AutoCertConfig.IsEmpty() {
			autoCertManager := AutocertManagerFromConfig(c.AutoCertConfig)
			tlsConfig = autoCertManager.TLSConfig()
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

	var tlsConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

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
