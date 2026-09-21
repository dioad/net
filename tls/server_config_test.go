package tls

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				InsecureSkipVerify: true, //nolint:gosec // asserting NewClientTLSConfig propagates the caller's opt-in InsecureSkipVerify setting
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
		SAN: SANConfig{
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

func TestNewLocalTLSConfigErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("single pem file load error is wrapped", func(t *testing.T) {
		_, err := NewLocalTLSConfig(ctx, LocalConfig{SinglePEMFile: filepath.Join(t.TempDir(), "missing.pem")})
		assert.ErrorContains(t, err, "error loading certificates from single pem file")
	})

	t.Run("certificate without key is rejected", func(t *testing.T) {
		_, err := NewLocalTLSConfig(ctx, LocalConfig{Certificate: "cert.pem"})
		assert.ErrorContains(t, err, "both certificate and key need to be specified")
	})

	t.Run("key without certificate is rejected", func(t *testing.T) {
		_, err := NewLocalTLSConfig(ctx, LocalConfig{Key: "key.pem"})
		assert.ErrorContains(t, err, "both certificate and key need to be specified")
	})

	t.Run("key pair load error is wrapped", func(t *testing.T) {
		dir := t.TempDir()
		_, err := NewLocalTLSConfig(ctx, LocalConfig{
			Certificate: filepath.Join(dir, "missing-cert.pem"),
			Key:         filepath.Join(dir, "missing-key.pem"),
		})
		require.Error(t, err)
		assert.ErrorContains(t, err, "error loading key pair and certs from files")
		assert.Error(t, errors.Unwrap(err), "the underlying error should be wrapped (%w), not just formatted as text")
	})
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
	caFile, err := os.Create(caPath) //nolint:gosec // caPath is a test-generated temp file, not external input
	if err != nil {
		t.Fatalf("Failed to create CA file: %v", err)
	}
	err = pem.Encode(caFile, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})
	if err != nil {
		t.Fatalf("Failed to write CA file: %v", err)
	}
	if err := caFile.Close(); err != nil {
		t.Fatalf("Failed to close CA file: %v", err)
	}

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
			name: "configFunc error is wrapped",
			c: ServerConfig{
				LocalConfig: LocalConfig{
					Certificate: filepath.Join(tempDir, "missing-cert.pem"),
					Key:         filepath.Join(tempDir, "missing-key.pem"),
				},
			},
			expectError: true,
		},
		{
			name: "with a single next proto, default is not appended",
			c: ServerConfig{
				LocalConfig: LocalConfig{Certificate: certPath, Key: keyPath},
				NextProtos:  []string{"custom-alpn"},
			},
			checkFunc: func(t *testing.T, got *tls.Config) {
				if !slices.Equal(got.NextProtos, []string{"custom-alpn"}) {
					t.Errorf("NextProtos = %v, want [custom-alpn]", got.NextProtos)
				}
			},
		},
		{
			// ACME tls-alpn-01's config already carries "acme-tls/1" in
			// NextProtos, so this is the only provider that exercises the
			// append-to-existing-NextProtos path rather than the
			// assign-when-empty path the other providers all take.
			name: "with ACME config, missing defaults are appended without duplicating existing entries",
			c: ServerConfig{
				ACME: ACMEConfig{
					Domains:        []string{"example.com"},
					CacheDirectory: t.TempDir(),
				},
			},
			checkFunc: func(t *testing.T, got *tls.Config) {
				// newAutocertTLSConfig already populates NextProtos with
				// "acme-tls/1", "h2" and "http/1.1" (in that order), so the
				// [h2, http/1.1] default is already fully covered and must
				// not be appended again as duplicates.
				base, err := newAutocertTLSConfig(ACMEConfig{Domains: []string{"example.com"}, CacheDirectory: t.TempDir()})
				if err != nil {
					t.Fatalf("newAutocertTLSConfig() error = %v", err)
				}

				if !slices.Equal(got.NextProtos, base.NextProtos) {
					t.Errorf("NextProtos = %v, want %v unchanged (h2 and http/1.1 already present)", got.NextProtos, base.NextProtos)
				}
				h2Count := 0
				for _, p := range got.NextProtos {
					if p == "h2" {
						h2Count++
					}
				}
				if h2Count != 1 {
					t.Errorf("NextProtos = %v, want exactly one [h2], got %d", got.NextProtos, h2Count)
				}
				if !slices.Contains(got.NextProtos, "acme-tls/1") {
					t.Errorf("NextProtos = %v, should still contain [acme-tls/1]", got.NextProtos)
				}
			},
		},
		{
			name: "client CA file read error is wrapped",
			c: ServerConfig{
				LocalConfig:  LocalConfig{Certificate: certPath, Key: keyPath},
				ClientCAFile: filepath.Join(tempDir, "missing-ca.pem"),
			},
			expectError: true,
		},
		{
			// A ServerConfig with only ClientCAFile set (no ACME/SelfSigned/
			// LocalConfig) makes configFuncFromConfig return nil, so
			// NewServerTLSConfig short-circuits to (nil, nil) before ever
			// reaching the client-CA handling below - LocalConfig is
			// required here so the code under test actually runs.
			name: "with client CA file",
			c:    ServerConfig{LocalConfig: LocalConfig{Certificate: certPath, Key: keyPath}, ClientCAFile: caPath},
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
			c: ServerConfig{
				LocalConfig:    LocalConfig{Certificate: certPath, Key: keyPath},
				ClientCAFile:   caPath,
				ClientAuthType: "RequireAndVerifyClientCert",
			},
			checkFunc: func(t *testing.T, got *tls.Config) {
				if got.ClientAuth != tls.RequireAndVerifyClientCert {
					t.Errorf("ClientAuth = %v, want %v", got.ClientAuth, tls.RequireAndVerifyClientCert)
				}
			},
		},
		{
			name: "with TLS min version",
			c: ServerConfig{
				LocalConfig:   LocalConfig{Certificate: certPath, Key: keyPath},
				TLSMinVersion: "TLS13",
			},
			checkFunc: func(t *testing.T, got *tls.Config) {
				if got.MinVersion != tls.VersionTLS13 {
					t.Errorf("MinVersion = %v, want %v", got.MinVersion, tls.VersionTLS13)
				}
			},
		},
		{
			name: "with server name",
			c: ServerConfig{
				LocalConfig: LocalConfig{Certificate: certPath, Key: keyPath},
				ServerName:  "example.com",
			},
			checkFunc: func(t *testing.T, got *tls.Config) {
				if got.ServerName != "example.com" {
					t.Errorf("ServerName = %v, want %v", got.ServerName, "example.com")
				}
			},
		},
		{
			name: "with next protos",
			c: ServerConfig{
				LocalConfig: LocalConfig{Certificate: certPath, Key: keyPath},
				NextProtos:  []string{"http/1.1", "h2c"},
			},
			checkFunc: func(t *testing.T, got *tls.Config) {
				if !slices.Contains(got.NextProtos, "http/1.1") {
					t.Errorf("NextProtos = %v, should contain [http/1.1]", got.NextProtos)
				}
				if !slices.Contains(got.NextProtos, "h2c") {
					t.Errorf("NextProtos = %v, should contain [h2c]", got.NextProtos)
				}
				// Since tlsConfig.NextProtos starts empty here, the "with
				// next protos" input replaces the default entirely (the
				// len==0 branch), so h2/http1.1 must NOT also be present.
				if slices.Contains(got.NextProtos, "h2") {
					t.Errorf("NextProtos = %v, should not contain the default [h2] when NextProtos is set", got.NextProtos)
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
				SelfSigned: SelfSignedConfig{
					CacheDirectory: tempDir,
					Subject:        CertificateSubject{CommonName: "test"},
					SAN: SANConfig{
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
				// Cases that supply their own NextProtos replace the
				// [h2, http/1.1] default entirely, so only check for it
				// when the case left NextProtos unset.
				if len(tt.c.NextProtos) == 0 {
					if !slices.Contains(got.NextProtos, "h2") {
						t.Errorf("NextProtos = %v, should contain [h2]", got.NextProtos)
					}
					if !slices.Contains(got.NextProtos, "http/1.1") {
						t.Errorf("NextProtos = %v, should contain [http/1.1]", got.NextProtos)
					}
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

func TestConfigFuncFromConfig_WarnsWhenMultipleArmsConfigured(t *testing.T) {
	tempDir := t.TempDir()
	cert, _ := helperCreateSelfSignedKeyPair(t, tempDir)
	certPath := filepath.Join(tempDir, "cert.pem")
	keyPath := filepath.Join(tempDir, "key.pem")
	require.NoError(t, SaveTLSCertificateToFiles(cert, certPath, keyPath))

	t.Run("only one arm configured logs nothing", func(t *testing.T) {
		var buf bytes.Buffer
		ctx := zerolog.New(&buf).WithContext(context.Background())

		configFuncFromConfig(ctx, ServerConfig{
			LocalConfig: LocalConfig{Certificate: certPath, Key: keyPath},
		})

		assert.Empty(t, buf.String())
	})

	t.Run("two arms configured logs a warning naming the winner", func(t *testing.T) {
		var buf bytes.Buffer
		ctx := zerolog.New(&buf).WithContext(context.Background())

		configFuncFromConfig(ctx, ServerConfig{
			SelfSigned: SelfSignedConfig{
				CacheDirectory: tempDir,
				Subject:        CertificateSubject{CommonName: "test"},
				Duration:       "1h",
				Bits:           1024,
			},
			LocalConfig: LocalConfig{Certificate: certPath, Key: keyPath},
		})

		logged := buf.String()
		assert.Contains(t, logged, "multiple TLS config arms")
		assert.Contains(t, logged, `"level":"warn"`)
	})
}
