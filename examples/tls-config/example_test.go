package tlsconfig_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	diohttp "github.com/dioad/net/http"
	diotls "github.com/dioad/net/tls"
)

// Example demonstrates TLS configuration for an HTTP server using self-signed certificates.
func Example() {
	// Create temporary directory for certificates
	tmpDir, err := os.MkdirTemp("", "tls-example-*")
	if err != nil {
		fmt.Printf("Error creating temp dir: %v\n", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	// Generate self-signed certificate
	selfSignedConfig := diotls.SelfSignedConfig{
		Subject: diotls.CertificateSubject{
			CommonName: "localhost",
		},
		SANConfig: diotls.SANConfig{
			DNSNames: []string{"localhost"},
		},
		Bits:     2048,   // RSA key size
		Duration: "24h",  // Certificate validity
	}

	_, _, err = diotls.CreateAndSaveSelfSignedKeyPair(selfSignedConfig, certFile, keyFile)
	if err != nil {
		fmt.Printf("Error generating certificate: %v\n", err)
		return
	}

	// Configure TLS with local certificate files
	tlsServerConfig := diotls.ServerConfig{
		LocalConfig: diotls.LocalConfig{
			Certificate: certFile,
			Key:         keyFile,
		},
	}

	tlsConfig, err := diotls.NewServerTLSConfig(context.Background(), tlsServerConfig)
	if err != nil {
		fmt.Printf("Error creating TLS config: %v\n", err)
		return
	}

	// Create server with TLS
	config := diohttp.Config{
		ListenAddress: ":443",
		TLSConfig:     tlsConfig,
	}
	server := diohttp.NewServer(config)

	fmt.Printf("Server configured with TLS: %t\n", server != nil)
	// Output: Server configured with TLS: true
}
