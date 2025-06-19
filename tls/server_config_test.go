package tls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestNewClientTLSConfig(t *testing.T) {
	baseCertFileName := "cert.pem"
	baseKeyFileName := "private.key"
	certFilePath := filepath.Join(t.TempDir(), baseCertFileName)
	keyFilePath := filepath.Join(t.TempDir(), baseKeyFileName)

	err := helperCreateCertificateWithSeparatePEMFiles(t, certFilePath, keyFilePath)
	if err != nil {
		t.Fatalf("helperCreateCertificateWithSinglePEMFiles() error = %v", err)
	}

	tests := []struct {
		name string
		c    ClientConfig
		want *tls.Config
	}{
		{
			name: "empty",
			c:    ClientConfig{},
			want: nil,
		},
		{
			name: "with certificate and key",
			c:    ClientConfig{Certificate: certFilePath, Key: keyFilePath},
			want: &tls.Config{
				MinVersion: tls.VersionTLS12,
				Certificates: []tls.Certificate{
					{
						Certificate: [][]byte{},
						PrivateKey:  nil,
					},
				},
				InsecureSkipVerify: false,
			},
		},
		{
			name: "with root CA file",
			c:    ClientConfig{RootCAFile: certFilePath},
			want: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				RootCAs:            nil,
				InsecureSkipVerify: false,
			},
		},
		{
			name: "with insecure skip verify",
			c:    ClientConfig{InsecureSkipVerify: true},
			want: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewClientTLSConfig(tt.c)
			if err != nil {
				t.Errorf("NewClientTLSConfig() error = %v", err)
			}
			if tt.want == nil && got == nil {
				return
			}

			// ignored for now until we have a way to test the generated certificate

		})
	}
}

func helperCreateSelfSignedKeyPair(t *testing.T, tempDir string) (*tls.Certificate, *x509.CertPool) {
	t.Helper()

	config := SelfSignedConfig{
		CacheDirectory: tempDir,
		Subject:        CertificateSubject{CommonName: t.Name()},
		SANConfig: SANConfig{
			DNSNames:    []string{"localhost"},
			IPAddresses: []string{"127.0.0.1"},
		},
		Duration: "5m",
		Bits:     1024,
	}

	cert, certPool, err := CreateSelfSignedKeyPair(config)
	if err != nil {
		t.Fatalf("CreateSelfSignedKeyPair() error = %v", err)
	}
	return cert, certPool
}

func helperCreateCertificateWithSeparatePEMFiles(t *testing.T, certPath, keyPath string) error {
	t.Helper()

	dirName := filepath.Dir(certPath)

	cert, _ := helperCreateSelfSignedKeyPair(t, dirName)

	return SaveTLSCertificateToFiles(cert, certPath, keyPath)
}

func helperCreateCertificateWithSinglePEMFiles(t *testing.T, filePath string) error {
	t.Helper()

	dirName := filepath.Dir(filePath)

	cert, _ := helperCreateSelfSignedKeyPair(t, dirName)

	return SaveTLSCertificateToFile(cert, filePath, 0644)
}

func TestNewLocalTLSConfig(t *testing.T) {
	testBaseFileName := "single-pem-file.pem"
	singleFilePath := filepath.Join(t.TempDir(), testBaseFileName)
	err := helperCreateCertificateWithSinglePEMFiles(t, singleFilePath)
	if err != nil {
		t.Fatalf("helperCreateCertificateWithSinglePEMFiles() error = %v", err)
	}

	tests := []struct {
		name string
		c    LocalConfig
		want *tls.Config
	}{
		{
			name: "single pem file",
			c:    LocalConfig{SinglePEMFile: singleFilePath},
			want: &tls.Config{Certificates: nil},
		},
		{
			name: "empty",
			c:    LocalConfig{},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewLocalTLSConfig(context.Background(), tt.c)
			if err != nil {
				t.Errorf("NewLocalTLSConfig() error = %v", err)
			}
			if tt.want == nil && got == nil {
				return
			}

			// ignored for now until we have a way to test the generated certificate
		})
	}
}

func TestNewServerTLSConfig(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	// Create a self-signed certificate for testing
	cert, _ := helperCreateSelfSignedKeyPair(t, tempDir)
	certPath := filepath.Join(tempDir, "cert.pem")
	keyPath := filepath.Join(tempDir, "key.pem")
	err := SaveTLSCertificateToFiles(cert, certPath, keyPath)
	if err != nil {
		t.Fatalf("Failed to save certificate: %v", err)
	}

	// Create a single PEM file for testing
	singlePEMPath := filepath.Join(tempDir, "single.pem")
	err = SaveTLSCertificateToFile(cert, singlePEMPath, 0644)
	if err != nil {
		t.Fatalf("Failed to save certificate to single PEM file: %v", err)
	}

	// Create a CA file for testing
	caPath := filepath.Join(tempDir, "ca.pem")
	caFile, err := os.Create(caPath)
	if err != nil {
		t.Fatalf("Failed to create CA file: %v", err)
	}
	err = pem.Encode(caFile, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})
	if err != nil {
		t.Fatalf("Failed to write CA file: %v", err)
	}
	caFile.Close()

	tests := []struct {
		name        string
		c           ServerConfig
		want        *tls.Config
		expectError bool
		checkFunc   func(*testing.T, *tls.Config)
	}{
		{
			name: "empty",
			c:    ServerConfig{},
			want: nil,
		},
		{
			name: "with client CA file",
			c:    ServerConfig{ClientCAFile: caPath},
			checkFunc: func(t *testing.T, got *tls.Config) {
				if got.ClientCAs == nil {
					t.Errorf("ClientCAs is nil, expected non-nil")
				}
				if got.ClientAuth != tls.NoClientCert {
					t.Errorf("ClientAuth = %v, want %v", got.ClientAuth, tls.NoClientCert)
				}
				if got.MinVersion != tls.VersionTLS12 {
					t.Errorf("MinVersion = %v, want %v", got.MinVersion, tls.VersionTLS12)
				}
			},
		},
		{
			name: "with client auth type",
			c:    ServerConfig{ClientCAFile: caPath, ClientAuthType: "RequireAndVerifyClientCert"},
			checkFunc: func(t *testing.T, got *tls.Config) {
				if got.ClientAuth != tls.RequireAndVerifyClientCert {
					t.Errorf("ClientAuth = %v, want %v", got.ClientAuth, tls.RequireAndVerifyClientCert)
				}
			},
		},
		{
			name: "with TLS min version",
			c:    ServerConfig{TLSMinVersion: "TLS13"},
			checkFunc: func(t *testing.T, got *tls.Config) {
				if got.MinVersion != tls.VersionTLS13 {
					t.Errorf("MinVersion = %v, want %v", got.MinVersion, tls.VersionTLS13)
				}
			},
		},
		{
			name: "with server name",
			c:    ServerConfig{ServerName: "example.com"},
			checkFunc: func(t *testing.T, got *tls.Config) {
				if got.ServerName != "example.com" {
					t.Errorf("ServerName = %v, want %v", got.ServerName, "example.com")
				}
			},
		},
		{
			name: "with next protos",
			c:    ServerConfig{NextProtos: []string{"http/1.1", "h2c"}},
			checkFunc: func(t *testing.T, got *tls.Config) {
				if !slices.Contains(got.NextProtos, "http/1.1") {
					t.Errorf("NextProtos = %v, should contain [http/1.1]", got.NextProtos)
				}
				if !slices.Contains(got.NextProtos, "h2c") {
					t.Errorf("NextProtos = %v, should contain [h2c]", got.NextProtos)
				}
			},
		},
		{
			name: "with local config",
			c: ServerConfig{
				LocalConfig: LocalConfig{
					SinglePEMFile: singlePEMPath,
				},
			},
			checkFunc: func(t *testing.T, got *tls.Config) {
				if len(got.Certificates) == 0 {
					t.Errorf("Certificates is empty, expected non-empty")
				}
			},
		},
		{
			name: "with self-signed config",
			c: ServerConfig{
				SelfSignedConfig: SelfSignedConfig{
					CacheDirectory: tempDir,
					Subject:        CertificateSubject{CommonName: "test"},
					SANConfig: SANConfig{
						DNSNames: []string{"localhost"},
					},
					Duration: "1h",
					Bits:     1024,
				},
			},
			checkFunc: func(t *testing.T, got *tls.Config) {
				if len(got.Certificates) == 0 {
					t.Errorf("Certificates is empty, expected non-empty")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewServerTLSConfig(context.Background(), tt.c)
			if tt.expectError {
				if err == nil {
					t.Errorf("NewServerTLSConfig() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("NewServerTLSConfig() error = %v", err)
				return
			}

			if tt.want == nil && got == nil {
				return
			}

			if got != nil {
				// Check default next protos
				if !slices.Contains(got.NextProtos, "h2") {
					t.Errorf("NextProtos = %v, should contain [h2]", got.NextProtos)
				}
				if !slices.Contains(got.NextProtos, "http/1.1") {
					t.Errorf("NextProtos = %v, should contain [http/1.1]", got.NextProtos)
				}

				// Run custom checks if provided
				if tt.checkFunc != nil {
					tt.checkFunc(t, got)
				}
			}
		})
	}
}

func TestNewSelfSignedTLSConfig(t *testing.T) {
	tests := []struct {
		name string
		c    SelfSignedConfig
		want *tls.Config
	}{
		{
			name: "empty",
			c:    SelfSignedConfig{},
			want: nil, // ignored for now until we have a way to test the generated certificate
		},
		{
			name: "non-empty",
			c: SelfSignedConfig{
				Alias:          "host",
				Duration:       "1h",
				CacheDirectory: t.TempDir(),
				Bits:           1024,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewSelfSignedTLSConfig(tt.c)
			if err != nil {
				t.Errorf("NewSelfSignedTLSConfig() error = %v", err)
			} else {
				if tt.want == nil && got == nil {
					return
				}

				// ignored for now until we have a way to test the generated certificate

			}
		})
	}
}
