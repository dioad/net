package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	diohttp "github.com/dioad/net/http"
	diotls "github.com/dioad/net/tls"
)

func main() {
	// Create temporary directory for certificates
	tmpDir, err := os.MkdirTemp("", "tls-example-*")
	if err != nil {
		log.Fatalf("Error creating temp dir: %v\n", err)
	}
	defer os.RemoveAll(tmpDir)

	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")

	fmt.Println("Generating self-signed certificate...")

	// Generate self-signed certificate
	selfSignedConfig := diotls.SelfSignedConfig{
		Subject: diotls.CertificateSubject{
			CommonName: "localhost",
		},
		SANConfig: diotls.SANConfig{
			DNSNames: []string{"localhost"},
		},
		Bits:     2048,  // RSA key size
		Duration: "24h", // Certificate validity
	}

	_, _, err = diotls.CreateAndSaveSelfSignedKeyPair(selfSignedConfig, certFile, keyFile)
	if err != nil {
		log.Fatalf("Error generating certificate: %v\n", err)
	}

	fmt.Printf("Certificate saved to: %s\n", certFile)
	fmt.Printf("Key saved to: %s\n", keyFile)

	// Configure TLS with local certificate files
	tlsServerConfig := diotls.ServerConfig{
		LocalConfig: diotls.LocalConfig{
			Certificate: certFile,
			Key:         keyFile,
		},
	}

	tlsConfig, err := diotls.NewServerTLSConfig(context.Background(), tlsServerConfig)
	if err != nil {
		log.Fatalf("Error creating TLS config: %v\n", err)
	}

	// Create a simple handler
	myHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from secure HTTPS server!\n")
	})

	// Create server with TLS
	config := diohttp.Config{
		ListenAddress: ":8443",
		TLSConfig:     tlsConfig,
	}
	server := diohttp.NewServer(config)
	server.AddHandler("/", myHandler)

	// Create listener
	ln, err := net.Listen("tcp", ":8443")
	if err != nil {
		log.Fatalf("Error creating listener: %v\n", err)
	}
	defer ln.Close()

	fmt.Println("\nStarting HTTPS server with TLS on :8443")
	fmt.Println("Note: Using self-signed certificate")
	fmt.Println("Try: curl -k https://localhost:8443/")
	fmt.Println("     (use -k to skip certificate verification)")

	// Start server in goroutine
	go func() {
		if err := server.Serve(ln); err != nil {
			log.Printf("Server error: %v\n", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down server...")
}
