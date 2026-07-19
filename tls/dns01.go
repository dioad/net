package tls

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-acme/lego/v5/acme"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/go-acme/lego/v5/certificate"
	"github.com/go-acme/lego/v5/challenge"
	"github.com/go-acme/lego/v5/lego"
	"github.com/go-acme/lego/v5/registration"
	"github.com/rs/zerolog"

	"github.com/dioad/generics"

	"github.com/dioad/util"
)

// dns01CertificateKeyType is the key algorithm used for issued certificates.
const dns01CertificateKeyType = certcrypto.RSA2048

// dns01RenewBefore is how long before expiry a certificate is reissued.
const dns01RenewBefore = 30 * 24 * time.Hour

// dns01TickEvery is how often the renewal loop checks whether the current
// certificate needs reissuing.
const dns01TickEvery = 24 * time.Hour

// dns01ObtainTimeout bounds lego's registration/order/finalize HTTP calls
// during a single ACME issuance attempt, in addition to ctx cancellation
// (which lego v5 also honours directly on these calls).
const dns01ObtainTimeout = 2 * time.Minute

// DNS01Config specifies parameters for obtaining and renewing a certificate
// via the ACME DNS-01 challenge.
//
// Provider cannot be populated via mapstructure/json decoding - it is a live
// object implementing github.com/go-acme/lego/v5/challenge.Provider, which
// presents and cleans up the _acme-challenge TXT record against whatever DNS
// hosting mechanism the caller uses. Callers should decode the remaining
// fields from configuration, then set Provider before passing the DNS01Config
// into ServerConfig / NewServerTLSConfig.
type DNS01Config struct {
	// Domains are the SANs to include on the certificate. May include
	// wildcard names (e.g. "*.example.com").
	Domains []string `mapstructure:"domains" json:",omitempty"`

	Email          string `mapstructure:"email" json:",omitempty"`
	DirectoryURL   string `mapstructure:"directory-url" json:",omitempty"`
	CacheDirectory string `mapstructure:"cache-directory" json:",omitempty"`

	// PropagationTimeout bounds how long lego waits for DNS propagation of
	// the challenge TXT record. Zero uses lego's default (60s).
	PropagationTimeout time.Duration `mapstructure:"propagation-timeout" json:",omitempty"`

	// PollingInterval controls how often lego polls while waiting for
	// propagation. Zero uses lego's default (2s).
	PollingInterval time.Duration `mapstructure:"polling-interval" json:",omitempty"`

	// Provider performs Present/CleanUp of the ACME DNS-01 challenge TXT
	// record. Must be set programmatically; it is not config-decodable.
	Provider challenge.Provider `mapstructure:"-" json:"-"`
}

// NewDNS01TLSConfigFunc creates a ConfigFunc for ACME DNS-01 certificate
// configuration. ctx bounds the lifetime of the background renewal
// goroutine started by NewDNS01TLSConfig, and is also passed to every ACME
// call lego makes on this arm's behalf.
func NewDNS01TLSConfigFunc(ctx context.Context, c DNS01Config) ConfigFunc {
	return func() (*tls.Config, error) { return NewDNS01TLSConfig(ctx, c) }
}

// NewDNS01TLSConfig obtains, or loads a cached, certificate via the ACME
// DNS-01 challenge and returns a TLS configuration whose GetCertificate
// serves the current certificate from memory. It starts a single background
// goroutine that periodically reissues the certificate as it approaches
// expiry; that goroutine exits when ctx is cancelled.
func NewDNS01TLSConfig(ctx context.Context, c DNS01Config) (*tls.Config, error) {
	if generics.IsZeroValue(c) {
		return nil, nil
	}
	if c.Provider == nil {
		return nil, fmt.Errorf("dns01: provider must be set")
	}
	if len(c.Domains) == 0 {
		return nil, fmt.Errorf("dns01: at least one domain must be configured")
	}

	mgr, err := newDNS01Manager(c)
	if err != nil {
		return nil, fmt.Errorf("error creating dns-01 manager: %w", err)
	}

	if err := mgr.ensureCertificate(ctx); err != nil {
		return nil, fmt.Errorf("error obtaining initial certificate: %w", err)
	}

	mgr.startRenewalLoop(ctx)

	return &tls.Config{
		MinVersion:     tls.VersionTLS12,
		GetCertificate: mgr.getCertificate,
	}, nil
}

// obtainedCert is the PEM-encoded result of one ACME issuance.
type obtainedCert struct {
	certPEM []byte
	keyPEM  []byte
}

// certObtainFunc performs one ACME DNS-01 issuance for c.Domains, using the
// account key at accountKeyPath (creating one if absent). Production code
// wires this to obtainViaLego; tests inject a fake that never touches the
// network.
type certObtainFunc func(ctx context.Context, c DNS01Config, accountKeyPath string) (*obtainedCert, error)

// dns01Manager holds runtime state for a single DNS01Config-based TLS
// config. A fresh manager is created per NewDNS01TLSConfig call, so no
// mutable state lives on DNS01Config itself.
type dns01Manager struct {
	config DNS01Config

	accountKeyPath string
	certPath       string
	keyPath        string

	obtain    certObtainFunc
	tickEvery time.Duration

	mu   sync.Mutex
	cert *tls.Certificate

	startOnce sync.Once
}

func newDNS01Manager(c DNS01Config) (*dns01Manager, error) {
	cacheDir, err := util.CreateDirPath(c.CacheDirectory, ".")
	if err != nil {
		return nil, fmt.Errorf("error creating cache directory: %w", err)
	}

	return &dns01Manager{
		config:         c,
		accountKeyPath: filepath.Join(cacheDir, "account.key"),
		certPath:       filepath.Join(cacheDir, "cert.pem"),
		keyPath:        filepath.Join(cacheDir, "cert.key"),
		obtain:         obtainViaLego,
		tickEvery:      dns01TickEvery,
	}, nil
}

// ensureCertificate loads a cached certificate if it exists and is not near
// expiry, otherwise it reissues one.
func (m *dns01Manager) ensureCertificate(ctx context.Context) error {
	if cert, ok := m.loadCachedCertificate(); ok && !needsRenewal(cert.Leaf.NotAfter, time.Now(), dns01RenewBefore) {
		m.setCertificate(cert)
		return nil
	}
	return m.reissue(ctx)
}

func (m *dns01Manager) reissue(ctx context.Context) error {
	obtained, err := m.obtain(ctx, m.config, m.accountKeyPath)
	if err != nil {
		return fmt.Errorf("error obtaining certificate via acme dns-01: %w", err)
	}

	if err := os.WriteFile(m.certPath, obtained.certPEM, 0644); err != nil {
		return fmt.Errorf("error persisting certificate: %w", err)
	}
	if err := os.WriteFile(m.keyPath, obtained.keyPEM, 0600); err != nil {
		return fmt.Errorf("error persisting private key: %w", err)
	}

	cert, err := tls.X509KeyPair(obtained.certPEM, obtained.keyPEM)
	if err != nil {
		return fmt.Errorf("error parsing obtained certificate: %w", err)
	}

	m.setCertificate(&cert)
	return nil
}

func (m *dns01Manager) loadCachedCertificate() (*tls.Certificate, bool) {
	cert, err := tls.LoadX509KeyPair(m.certPath, m.keyPath)
	if err != nil {
		return nil, false
	}

	if cert.Leaf == nil {
		leaf, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return nil, false
		}
		cert.Leaf = leaf
	}

	return &cert, true
}

// needsRenewal reports whether a certificate expiring at notAfter should be
// reissued given the current time and how long before expiry to renew.
func needsRenewal(notAfter, now time.Time, renewBefore time.Duration) bool {
	return !now.Before(notAfter.Add(-renewBefore))
}

func (m *dns01Manager) getCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cert == nil {
		return nil, fmt.Errorf("dns-01: no certificate available")
	}
	return m.cert, nil
}

func (m *dns01Manager) setCertificate(cert *tls.Certificate) {
	m.mu.Lock()
	m.cert = cert
	m.mu.Unlock()
}

// startRenewalLoop starts the background renewal goroutine exactly once.
func (m *dns01Manager) startRenewalLoop(ctx context.Context) {
	m.startOnce.Do(func() {
		go m.renewalLoop(ctx)
	})
}

// renewalLoop periodically reissues the certificate as it approaches
// expiry. It exits when ctx is cancelled; ctx is also passed into every ACME
// call lego makes for a renewal attempt, so lego v5 propagates cancellation
// directly into any in-flight registration/order/finalize/propagation-wait
// call rather than relying solely on dns01ObtainTimeout.
func (m *dns01Manager) renewalLoop(ctx context.Context) {
	ticker := time.NewTicker(m.tickEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.renewIfDue(ctx); err != nil {
				zerolog.Ctx(ctx).Error().Err(err).Msg("dns-01: certificate renewal failed, will retry next tick")
			}
		}
	}
}

// renewIfDue reissues the certificate if the currently served in-memory
// certificate is near expiry. The renewal decision is based on the served
// certificate, not a re-read of the on-disk cache, so an external change to
// the cache files (deletion, corruption) cannot trigger a needless
// reissue - and burn ACME rate-limit budget - while a valid certificate is
// still being served.
func (m *dns01Manager) renewIfDue(ctx context.Context) error {
	m.mu.Lock()
	cert := m.cert
	m.mu.Unlock()

	if cert != nil && !needsRenewal(cert.Leaf.NotAfter, time.Now(), dns01RenewBefore) {
		return nil
	}
	return m.reissue(ctx)
}

// dns01ProviderWithTimeout adapts a challenge.Provider to also satisfy
// challenge.ProviderTimeout, so that DNS01Config's PropagationTimeout and
// PollingInterval reach lego's DNS-01 propagation retry loop regardless of
// whether the caller's own Provider implementation exposes a Timeout method.
type dns01ProviderWithTimeout struct {
	challenge.Provider
	timeout  time.Duration
	interval time.Duration
}

func (p dns01ProviderWithTimeout) Timeout() (time.Duration, time.Duration) {
	return p.timeout, p.interval
}

// dns01User implements registration.User for the ACME account key.
type dns01User struct {
	email string
	key   *ecdsa.PrivateKey
	reg   *acme.ExtendedAccount
}

func (u *dns01User) GetEmail() string                       { return u.email }
func (u *dns01User) GetRegistration() *acme.ExtendedAccount { return u.reg }
func (u *dns01User) GetPrivateKey() crypto.Signer           { return u.key }

// obtainViaLego runs the ACME DNS-01 flow (client creation, provider
// registration, account registration, certificate issuance) for c.Domains,
// producing PEM-encoded certificate and private key material.
func obtainViaLego(ctx context.Context, c DNS01Config, accountKeyPath string) (*obtainedCert, error) {
	accountKey, err := loadOrCreateAccountKey(accountKeyPath)
	if err != nil {
		return nil, fmt.Errorf("error loading acme account key: %w", err)
	}

	user := &dns01User{email: c.Email, key: accountKey}

	legoConfig := lego.NewConfig(user)
	if c.DirectoryURL != "" {
		legoConfig.CADirURL = c.DirectoryURL
	}
	legoConfig.Certificate.Timeout = dns01ObtainTimeout

	client, err := lego.NewClient(legoConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating acme client: %w", err)
	}

	provider := c.Provider
	if c.PropagationTimeout > 0 || c.PollingInterval > 0 {
		timeout, interval := c.PropagationTimeout, c.PollingInterval
		if timeout == 0 {
			timeout = 60 * time.Second
		}
		if interval == 0 {
			interval = 2 * time.Second
		}
		provider = dns01ProviderWithTimeout{Provider: c.Provider, timeout: timeout, interval: interval}
	}

	if err := client.Challenge.SetDNS01Provider(provider); err != nil {
		return nil, fmt.Errorf("error setting dns-01 provider: %w", err)
	}

	reg, err := client.Registration.Register(ctx, registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return nil, fmt.Errorf("error registering acme account: %w", err)
	}
	user.reg = reg

	resource, err := client.Certificate.Obtain(ctx, certificate.ObtainRequest{
		Domains: c.Domains,
		KeyType: dns01CertificateKeyType,
		Bundle:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("error obtaining certificate: %w", err)
	}

	return &obtainedCert{certPEM: resource.Certificate, keyPEM: resource.PrivateKey}, nil
}

// loadOrCreateAccountKey loads the ACME account private key from path,
// creating and persisting a new one if it doesn't exist.
func loadOrCreateAccountKey(path string) (*ecdsa.PrivateKey, error) {
	pathClean := filepath.Clean(path)
	if data, err := os.ReadFile(pathClean); err == nil {
		block, _ := pem.Decode(data)
		if block == nil {
			return nil, fmt.Errorf("invalid PEM in account key file %s", path)
		}
		return x509.ParseECPrivateKey(block.Bytes)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("error reading account key: %w", err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("error generating account key: %w", err)
	}

	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("error marshaling account key: %w", err)
	}

	if err := saveBlockToPEMFile(path, 0600, "EC PRIVATE KEY", der); err != nil {
		return nil, fmt.Errorf("error saving account key: %w", err)
	}

	return key, nil
}
