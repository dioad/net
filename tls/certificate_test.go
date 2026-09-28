package tls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadX509CertFromFile_InvalidPEM(t *testing.T) {
	// Create a temporary file with invalid PEM data
	tmpFile, err := os.CreateTemp(t.TempDir(), "invalid_pem")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.WriteString("not a pem"); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}

	// This should not panic
	_, err = LoadX509CertFromFile(tmpFile.Name())
	if err == nil {
		t.Error("expected an error for invalid PEM data, got nil")
	}
}

func TestSaveTLSCertificateToFile(t *testing.T) {
	t.Run("returns an error when the file cannot be opened", func(t *testing.T) {
		// A directory path can't be opened for writing as a regular file.
		dir := t.TempDir()
		cert := &tls.Certificate{Certificate: [][]byte{{0x01}}, PrivateKey: "irrelevant"}

		err := SaveTLSCertificateToFile(cert, dir, 0644)
		assert.ErrorContains(t, err, "is a directory")
	})

	t.Run("returns an error when the private key cannot be marshalled", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "cert.pem")
		cert := &tls.Certificate{
			Certificate: [][]byte{{0x01, 0x02, 0x03}},
			PrivateKey:  "not-a-supported-key-type",
		}

		err := SaveTLSCertificateToFile(cert, path, 0644)
		require.Error(t, err)

		// The certificate block is written before the private key is
		// marshalled, so it should already be on disk despite the error.
		content, readErr := os.ReadFile(path) //nolint:gosec // path is a test-generated temp file, not external input
		require.NoError(t, readErr)
		assert.Contains(t, string(content), "-----BEGIN CERTIFICATE-----")
	})
}

func TestSaveTLSCertificateToFiles(t *testing.T) {
	t.Run("writes the certificate and key with the documented permissions", func(t *testing.T) {
		dir := t.TempDir()
		cert, _ := helperCreateSelfSignedKeyPair(t, dir)
		certPath := filepath.Join(dir, "cert.pem")
		keyPath := filepath.Join(dir, "key.pem")

		require.NoError(t, SaveTLSCertificateToFiles(cert, certPath, keyPath))

		certInfo, err := os.Stat(certPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0644), certInfo.Mode())

		keyInfo, err := os.Stat(keyPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), keyInfo.Mode())
	})

	t.Run("returns an error when the certificate file cannot be opened", func(t *testing.T) {
		// A directory path can't be opened for writing as a regular file.
		certDir := t.TempDir()
		keyPath := filepath.Join(t.TempDir(), "key.pem")
		cert := &tls.Certificate{Certificate: [][]byte{{0x01}}, PrivateKey: "irrelevant"}

		err := SaveTLSCertificateToFiles(cert, certDir, keyPath)
		require.ErrorContains(t, err, "is a directory")

		_, statErr := os.Stat(keyPath)
		assert.True(t, os.IsNotExist(statErr), "the key file should not be written when the certificate file fails to open")
	})

	t.Run("returns an error when the private key cannot be marshalled", func(t *testing.T) {
		certPath := filepath.Join(t.TempDir(), "cert.pem")
		keyPath := filepath.Join(t.TempDir(), "key.pem")
		cert := &tls.Certificate{
			Certificate: [][]byte{{0x01, 0x02, 0x03}},
			PrivateKey:  "not-a-supported-key-type",
		}

		err := SaveTLSCertificateToFiles(cert, certPath, keyPath)
		require.Error(t, err)

		content, readErr := os.ReadFile(certPath) //nolint:gosec // certPath is a test-generated temp file, not external input
		require.NoError(t, readErr)
		assert.Contains(t, string(content), "-----BEGIN CERTIFICATE-----")

		_, statErr := os.Stat(keyPath)
		assert.True(t, os.IsNotExist(statErr), "the key file should not be written when the private key fails to marshal")
	})
}

func TestLoadKeyPairAndCertsFromFile(t *testing.T) {
	t.Run("returns an error when the file cannot be read", func(t *testing.T) {
		_, err := LoadKeyPairAndCertsFromFile(filepath.Join(t.TempDir(), "missing.pem"))
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("returns an error for an unparseable private key block", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "bad-key.pem")
		data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("not a real key")})
		require.NoError(t, os.WriteFile(path, data, 0600))

		_, err := LoadKeyPairAndCertsFromFile(path)
		assert.ErrorContains(t, err, "failure reading private key")
	})

	t.Run("returns an error when no certificate block is present", func(t *testing.T) {
		key, err := rsa.GenerateKey(rand.Reader, 1024) //nolint:gosec // small key generated only for test speed; strength is irrelevant to this test
		require.NoError(t, err)
		path := filepath.Join(t.TempDir(), "key-only.pem")
		data := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
		require.NoError(t, os.WriteFile(path, data, 0600))

		_, err = LoadKeyPairAndCertsFromFile(path)
		assert.ErrorContains(t, err, "no certificate found")
	})

	t.Run("returns an error when no private key block is present", func(t *testing.T) {
		cert, _ := helperCreateSelfSignedKeyPair(t, t.TempDir())
		path := filepath.Join(t.TempDir(), "cert-only.pem")
		data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})
		require.NoError(t, os.WriteFile(path, data, 0600))

		_, err := LoadKeyPairAndCertsFromFile(path)
		assert.ErrorContains(t, err, "no private key found")
	})
}
