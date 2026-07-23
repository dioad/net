package tls

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-acme/lego/v5/acme"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/go-acme/lego/v5/certificate"
	"github.com/go-acme/lego/v5/challenge"
	"github.com/go-acme/lego/v5/lego"
	"github.com/go-acme/lego/v5/registration"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/acme/autocert"

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

// domainSetCacheKey derives a stable, filesystem- and autocert.Cache-safe
// identifier for a set of domains. It is order-independent (the domains are
// sorted before hashing) so reissuing under a reordered but otherwise
// identical Domains list reuses the same cache entry, and it never contains
// characters autocert.Cache's contract forbids (notably "*"), so wildcard
// domains such as "*.example.com" are safe to include. Deriving the cache
// key from the domain set - rather than a fixed name - is what lets multiple
// ACMEConfig instances for different domains share one CacheDirectory
// without their certificates and keys colliding.
func domainSetCacheKey(domains []string) string {
	sorted := make([]string, len(domains))
	for i, d := range domains {
		sorted[i] = strings.ToLower(d)
	}
	sort.Strings(sorted)

	sum := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	return hex.EncodeToString(sum[:])[:16]
}

// accountCacheKey derives a stable cache key for the ACME account tied to
// email and directoryURL. The account key is a distinct identity from any
// certificate's domain set, so ACMEConfig instances that share a
// CacheDirectory but use different accounts (different email or ACME
// directory) get separate account keys automatically, while instances
// sharing the same account reuse one.
func accountCacheKey(email, directoryURL string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(email) + "\n" + directoryURL))
	return hex.EncodeToString(sum[:])[:16]
}

// newDNS01TLSConfig obtains, or loads a cached, certificate via the ACME
// dns-01 challenge and returns a TLS configuration whose GetCertificate
// serves the current certificate from memory. It starts a single background
// goroutine that periodically reissues the certificate as it approaches
// expiry; that goroutine exits when ctx is cancelled.
func newDNS01TLSConfig(ctx context.Context, c ACMEConfig) (*tls.Config, error) {
	if c.DNS01.Provider == nil {
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

// certObtainFunc performs one ACME dns-01 issuance for c.Domains, using the
// account key cached under accountKeyName (creating one if absent).
// Production code wires this to obtainViaLego; tests inject a fake that
// never touches the network.
type certObtainFunc func(ctx context.Context, c ACMEConfig, cache autocert.Cache, accountKeyName string) (*obtainedCert, error)

// dns01Manager holds runtime state for a single ACMEConfig-based dns-01 TLS
// config. A fresh manager is created per newDNS01TLSConfig call, so no
// mutable state lives on ACMEConfig itself.
type dns01Manager struct {
	config ACMEConfig

	// cache stores the ACME account key and issued certificate/key material,
	// keyed by accountKeyName/certName/certKeyName. It is the same Cache
	// abstraction (golang.org/x/crypto/acme/autocert.Cache) that the
	// autocert TLS arm uses, so a CacheDirectory can be shared between them.
	cache          autocert.Cache
	accountKeyName string
	certName       string
	certKeyName    string

	obtain    certObtainFunc
	tickEvery time.Duration

	mu   sync.Mutex
	cert *tls.Certificate

	startOnce sync.Once
}

func newDNS01Manager(c ACMEConfig) (*dns01Manager, error) {
	cacheDir, err := util.CreateDirPath(c.CacheDirectory, ".")
	if err != nil {
		return nil, fmt.Errorf("error creating cache directory: %w", err)
	}

	certKey := domainSetCacheKey(c.Domains)
	accountKey := accountCacheKey(c.Email, c.DirectoryURL)

	return &dns01Manager{
		config:         c,
		cache:          autocert.DirCache(cacheDir),
		accountKeyName: accountKey + "+account",
		certName:       certKey + "+crt",
		certKeyName:    certKey + "+key",
		obtain:         obtainViaLego,
		tickEvery:      dns01TickEvery,
	}, nil
}

// ensureCertificate loads a cached certificate if it exists and is not near
// expiry, otherwise it reissues one.
func (m *dns01Manager) ensureCertificate(ctx context.Context) error {
	if cert, ok := m.loadCachedCertificate(ctx); ok && !needsRenewal(cert.Leaf.NotAfter, time.Now(), dns01RenewBefore) {
		m.setCertificate(cert)
		return nil
	}
	return m.reissue(ctx)
}

func (m *dns01Manager) reissue(ctx context.Context) error {
	obtained, err := m.obtainWithContext(ctx)
	if err != nil {
		return fmt.Errorf("error obtaining certificate via acme dns-01: %w", err)
	}

	if err := m.cache.Put(ctx, m.certName, obtained.certPEM); err != nil {
		return fmt.Errorf("error persisting certificate: %w", err)
	}
	if err := m.cache.Put(ctx, m.certKeyName, obtained.keyPEM); err != nil {
		return fmt.Errorf("error persisting private key: %w", err)
	}

	cert, err := tls.X509KeyPair(obtained.certPEM, obtained.keyPEM)
	if err != nil {
		return fmt.Errorf("error parsing obtained certificate: %w", err)
	}

	m.setCertificate(&cert)
	return nil
}

// dns01ObtainResult carries the outcome of a background m.obtain call so
// obtainWithContext can select between it and ctx cancellation.
type dns01ObtainResult struct {
	cert *obtainedCert
	err  error
}

// obtainWithContext runs m.obtain in a separate goroutine and returns as
// soon as either it completes or ctx is cancelled. lego v5's DNS-01
// propagation-wait loop (internal/wait.For) takes no context and ignores
// cancellation while polling for record propagation, so a plain blocking
// call to m.obtain would keep reissue running past ctx cancellation for up
// to the propagation timeout. The obtain goroutine is not killed when ctx
// is cancelled - it holds no lock and has no side effect beyond its own
// ACME network calls - so it is left to finish or fail on its own; the
// buffered channel ensures it never blocks trying to send its result.
func (m *dns01Manager) obtainWithContext(ctx context.Context) (*obtainedCert, error) {
	resultCh := make(chan dns01ObtainResult, 1)
	go func() {
		cert, err := m.obtain(ctx, m.config, m.cache, m.accountKeyName)
		resultCh <- dns01ObtainResult{cert: cert, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		return result.cert, result.err
	}
}

func (m *dns01Manager) loadCachedCertificate(ctx context.Context) (*tls.Certificate, bool) {
	certPEM, err := m.cache.Get(ctx, m.certName)
	if err != nil {
		return nil, false
	}
	keyPEM, err := m.cache.Get(ctx, m.certKeyName)
	if err != nil {
		return nil, false
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
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

// obtainViaLego runs the ACME dns-01 flow (client creation, provider
// registration, account registration, certificate issuance) for c.Domains,
// producing PEM-encoded certificate and private key material.
func obtainViaLego(ctx context.Context, c ACMEConfig, cache autocert.Cache, accountKeyName string) (*obtainedCert, error) {
	accountKey, err := loadOrCreateAccountKey(ctx, cache, accountKeyName)
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

	provider := c.DNS01.Provider
	if c.DNS01.PropagationTimeout > 0 || c.DNS01.PollingInterval > 0 {
		timeout, interval := c.DNS01.PropagationTimeout, c.DNS01.PollingInterval
		if timeout == 0 {
			timeout = 60 * time.Second
		}
		if interval == 0 {
			interval = 2 * time.Second
		}
		provider = dns01ProviderWithTimeout{Provider: c.DNS01.Provider, timeout: timeout, interval: interval}
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

// loadOrCreateAccountKey loads the ACME account private key cached under
// key, creating and persisting a new one if it doesn't exist.
func loadOrCreateAccountKey(ctx context.Context, cache autocert.Cache, key string) (*ecdsa.PrivateKey, error) {
	if data, err := cache.Get(ctx, key); err == nil {
		block, _ := pem.Decode(data)
		if block == nil {
			return nil, fmt.Errorf("invalid PEM in account key %s", key)
		}
		return x509.ParseECPrivateKey(block.Bytes)
	} else if !errors.Is(err, autocert.ErrCacheMiss) {
		return nil, fmt.Errorf("error reading account key: %w", err)
	}

	accountKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("error generating account key: %w", err)
	}

	der, err := x509.MarshalECPrivateKey(accountKey)
	if err != nil {
		return nil, fmt.Errorf("error marshaling account key: %w", err)
	}

	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	if err := cache.Put(ctx, key, pemBytes); err != nil {
		return nil, fmt.Errorf("error saving account key: %w", err)
	}

	return accountKey, nil
}
