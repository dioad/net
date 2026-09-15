package tls

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadX509CertFromFile_InvalidPEM(t *testing.T) {
	// Create a temporary file with invalid PEM data
	tmpFile, err := os.CreateTemp("", "invalid_pem")
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
		assert.Error(t, err)

		// The certificate block is written before the private key is
		// marshalled, so it should already be on disk despite the error.
		content, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Contains(t, string(content), "-----BEGIN CERTIFICATE-----")
	})
}
