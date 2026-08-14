package dns_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dioad/net/dns"
)

type tlsaRenderData struct {
	Domain string
}

// writeDANECertFixture writes a fresh self-signed certificate for mxDomain
// under stateDir/autocert/mxDomain/cert.pem, matching the layout a
// "{{.Domain}}"-templated CertPathTemplate resolves to.
func writeDANECertFixture(t *testing.T, stateDir, mxDomain string) {
	t.Helper()

	dir := filepath.Join(stateDir, "autocert", mxDomain)
	require.NoError(t, os.MkdirAll(dir, 0o755))

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: mxDomain},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cert.pem"), certPEM, 0o600))
}

func newTLSARecord(stateDir string) *dns.TLSARecord {
	return &dns.TLSARecord{
		CertPathTemplate: filepath.Join(stateDir, "autocert", "mx.{{.Domain}}", "cert.pem"),
		Name:             "mx",
		Port:             25,
		Proto:            "tcp",
		MatchingType:     1,
		Usage:            1,
	}
}

func TestTLSARecord_RecordPrefix(t *testing.T) {
	t.Parallel()

	r := &dns.TLSARecord{Name: "mx", Port: 25, Proto: "tcp"}
	assert.Equal(t, "_25._tcp.mx.", r.RecordPrefix())
}

func TestTLSARecord_RecordType(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "TLSA", (&dns.TLSARecord{}).RecordType())
}

func TestTLSARecord_Render_FetchesOnce(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	writeDANECertFixture(t, stateDir, "mx.example.com")

	r := newTLSARecord(stateDir)
	data := tlsaRenderData{Domain: "example.com"}

	require.NoError(t, r.Render(data))
	first := r.RecordValue()
	require.NotEmpty(t, first)

	// A fresh cert at the same path would produce a different TLSA value;
	// Render's fetch-once guard means a second call must not re-derive it.
	writeDANECertFixture(t, stateDir, "mx.example.com")
	require.NoError(t, r.Render(data))

	assert.Equal(t, first, r.RecordValue())
}

// TestTLSARecord_Render_AutoRefresh_NoRace exercises the AutoRefresh path
// under -race: fetchDNSContents writes the cached value from the ticker
// goroutine while RecordValue/String read it concurrently from the test
// goroutine.
func TestTLSARecord_Render_AutoRefresh_NoRace(t *testing.T) {
	stateDir := t.TempDir()
	writeDANECertFixture(t, stateDir, "mx.example.com")

	r := newTLSARecord(stateDir)
	r.AutoRefresh = true
	r.AutoRefreshPeriodSeconds = 1
	data := tlsaRenderData{Domain: "example.com"}

	require.NoError(t, r.Render(data))

	deadline := time.After(1500 * time.Millisecond)
	for {
		select {
		case <-deadline:
			assert.NotEmpty(t, r.RecordValue())
			assert.NotEmpty(t, r.String())
			return
		default:
			_ = r.RecordValue()
			_ = r.String()
		}
	}
}
