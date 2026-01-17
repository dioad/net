package tls

import (
	"os"
	"testing"
)

func TestLoadX509CertFromFile_InvalidPEM(t *testing.T) {
	// Create a temporary file with invalid PEM data
	tmpFile, err := os.CreateTemp("", "invalid_pem")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

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
