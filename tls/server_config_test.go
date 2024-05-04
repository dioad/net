package tls

import (
	"crypto/tls"
	"crypto/x509"
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
			got, err := NewLocalTLSConfig(tt.c)
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
	tests := []struct {
		name string
		c    ServerConfig
		want *tls.Config
	}{
		{
			name: "empty",
			c:    ServerConfig{},
			want: nil,
		},
		{
			name: "with client CA file",
			c:    ServerConfig{ClientCAFile: "client-ca-file"},
			// want: &tls.Config{
			// 	MinVersion: tls.VersionTLS12,
			// 	ClientAuth: tls.NoClientCert,
			// 	ClientCAs:  nil,
			// },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewServerTLSConfig(tt.c)
			if err != nil {
				t.Errorf("NewServerTLSConfig() error = %v", err)
			} else {
				if tt.want == nil && got == nil {
					return
				}
				if got != nil {
					if !slices.Contains(got.NextProtos, "h2") {
						t.Errorf("NewAutocertTLSConfig() NextProtos = %v, should contain [h2]", got.NextProtos)
					}

					// ignored for now until we have a way to test the generated certificate
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
