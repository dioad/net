package tls

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDNS01Provider struct{}

func (fakeDNS01Provider) Present(_ context.Context, _, _, _ string) error { return nil }
func (fakeDNS01Provider) CleanUp(_ context.Context, _, _, _ string) error { return nil }

func genTestCertPEM(t *testing.T, notAfter time.Time) (certPEM, keyPEM []byte) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	serialNumber, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "dns01-test",
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  notAfter,
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	return certPEM, keyPEM
}

func TestNeedsRenewal(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	renewBefore := 30 * 24 * time.Hour

	tests := []struct {
		name     string
		notAfter time.Time
		want     bool
	}{
		{"well before renewal window", now.Add(60 * 24 * time.Hour), false},
		{"exactly at renewal threshold", now.Add(renewBefore), true},
		{"already expired", now.Add(-time.Hour), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, needsRenewal(tt.notAfter, now, renewBefore))
		})
	}
}

func TestNewDNS01TLSConfig(t *testing.T) {
	ctx := context.Background()

	t.Run("zero value returns nil", func(t *testing.T) {
		cfg, err := NewDNS01TLSConfig(ctx, DNS01Config{})
		require.NoError(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("missing provider is an error", func(t *testing.T) {
		_, err := NewDNS01TLSConfig(ctx, DNS01Config{Domains: []string{"example.com"}})
		assert.Error(t, err)
	})

	t.Run("missing domains is an error", func(t *testing.T) {
		_, err := NewDNS01TLSConfig(ctx, DNS01Config{Provider: fakeDNS01Provider{}})
		assert.Error(t, err)
	})
}

func TestConfigFuncFromConfigSelectsDNS01(t *testing.T) {
	ctx := context.Background()
	cfg := ServerConfig{
		DNS01: DNS01Config{
			Domains:  []string{"example.com"},
			Provider: fakeDNS01Provider{},
		},
	}

	configFunc := configFuncFromConfig(ctx, cfg)
	assert.NotNil(t, configFunc)
}

func TestDNS01ManagerEnsureCertificateCacheHit(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "cert.key")

	certPEM, keyPEM := genTestCertPEM(t, time.Now().Add(90*24*time.Hour))
	require.NoError(t, os.WriteFile(certPath, certPEM, 0600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0600))

	var calls int
	m := &dns01Manager{
		certPath: certPath,
		keyPath:  keyPath,
		obtain: func(context.Context, DNS01Config, string) (*obtainedCert, error) {
			calls++
			return nil, assert.AnError
		},
	}

	require.NoError(t, m.ensureCertificate(context.Background()))
	assert.Equal(t, 0, calls, "obtain should not be called when the cached certificate is not near expiry")

	cert, err := m.getCertificate(nil)
	require.NoError(t, err)
	assert.NotNil(t, cert)
}

func TestDNS01ManagerEnsureCertificateReissuesWhenNearExpiry(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "cert.key")

	oldCertPEM, oldKeyPEM := genTestCertPEM(t, time.Now().Add(time.Hour))
	require.NoError(t, os.WriteFile(certPath, oldCertPEM, 0600))
	require.NoError(t, os.WriteFile(keyPath, oldKeyPEM, 0600))

	newCertPEM, newKeyPEM := genTestCertPEM(t, time.Now().Add(90*24*time.Hour))

	var calls int
	m := &dns01Manager{
		certPath: certPath,
		keyPath:  keyPath,
		obtain: func(context.Context, DNS01Config, string) (*obtainedCert, error) {
			calls++
			return &obtainedCert{certPEM: newCertPEM, keyPEM: newKeyPEM}, nil
		},
	}

	require.NoError(t, m.ensureCertificate(context.Background()))
	assert.Equal(t, 1, calls)

	persistedCertPEM, err := os.ReadFile(certPath)
	require.NoError(t, err)
	assert.Equal(t, newCertPEM, persistedCertPEM)
}

func TestDNS01ManagerRenewalLoopExitsOnContextCancellation(t *testing.T) {
	dir := t.TempDir()

	// Always return a soon-expiring certificate so every tick reissues,
	// giving a deterministic call count to poll for.
	freshCertPEM, freshKeyPEM := genTestCertPEM(t, time.Now().Add(time.Hour))

	var calls atomic.Int32
	m := &dns01Manager{
		certPath:  filepath.Join(dir, "cert.pem"),
		keyPath:   filepath.Join(dir, "cert.key"),
		tickEvery: 10 * time.Millisecond,
		obtain: func(context.Context, DNS01Config, string) (*obtainedCert, error) {
			calls.Add(1)
			return &obtainedCert{certPEM: freshCertPEM, keyPEM: freshKeyPEM}, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		m.renewalLoop(ctx)
		close(done)
	}()

	require.Eventually(t, func() bool {
		return calls.Load() >= 2
	}, time.Second, time.Millisecond, "expected renewal loop to reissue repeatedly")

	cert, err := m.getCertificate(nil)
	require.NoError(t, err)
	assert.NotNil(t, cert)

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("renewal loop goroutine did not exit after context cancellation")
	}
}

func TestDNS01ManagerReissueReturnsPromptlyOnContextCancellation(t *testing.T) {
	dir := t.TempDir()

	unblock := make(chan struct{})
	obtainStarted := make(chan struct{})
	m := &dns01Manager{
		certPath: filepath.Join(dir, "cert.pem"),
		keyPath:  filepath.Join(dir, "cert.key"),
		obtain: func(ctx context.Context, _ DNS01Config, _ string) (*obtainedCert, error) {
			close(obtainStarted)
			<-unblock
			return nil, ctx.Err()
		},
	}
	t.Cleanup(func() { close(unblock) })

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- m.reissue(ctx)
	}()

	<-obtainStarted
	cancel()

	select {
	case err := <-errCh:
		assert.ErrorIs(t, err, context.Canceled, "reissue should return the context error, not wait for obtain")
	case <-time.After(time.Second):
		t.Fatal("reissue did not return promptly after context cancellation")
	}
}

func TestLoadOrCreateAccountKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "account.key")

	key1, err := loadOrCreateAccountKey(path)
	require.NoError(t, err)
	require.NotNil(t, key1)

	key2, err := loadOrCreateAccountKey(path)
	require.NoError(t, err)

	der1, err := x509.MarshalECPrivateKey(key1)
	require.NoError(t, err)
	der2, err := x509.MarshalECPrivateKey(key2)
	require.NoError(t, err)
	assert.Equal(t, der1, der2, "second call should load the persisted key rather than generating a new one")
}

func TestDNS01ProviderWithTimeoutReturnsConfiguredValues(t *testing.T) {
	p := dns01ProviderWithTimeout{
		Provider: fakeDNS01Provider{},
		timeout:  5 * time.Minute,
		interval: 10 * time.Second,
	}

	timeout, interval := p.Timeout()
	assert.Equal(t, 5*time.Minute, timeout)
	assert.Equal(t, 10*time.Second, interval)
}
