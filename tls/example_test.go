package tls_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	diotls "github.com/dioad/net/tls"
)

// selfSigned describes a certificate for a server reached at 127.0.0.1.
var selfSigned = diotls.SelfSignedConfig{
	Subject:  diotls.CertificateSubject{CommonName: "localhost"},
	SAN:      diotls.SANConfig{DNSNames: []string{"localhost"}, IPAddresses: []string{"127.0.0.1"}},
	Bits:     2048,
	Duration: "1h",
}

// A ServerConfig names one source for the server's certificate: ACME, a
// self-signed certificate made at startup, or local files. This one is
// self-signed.
func ExampleNewServerTLSConfig() {
	tlsConfig, err := diotls.NewServerTLSConfig(context.Background(), diotls.ServerConfig{
		SelfSigned:    selfSigned,
		TLSMinVersion: "TLS13",
	})
	if err != nil {
		fmt.Println("server tls config:", err)

		return
	}

	fmt.Println("minimum version:", tls.VersionName(tlsConfig.MinVersion))
	// Output:
	// minimum version: TLS 1.3
}

// A server that reads its certificate from files, and a client that trusts
// it through a root CA file.
func ExampleNewClientTLSConfig() {
	dir, err := os.MkdirTemp("", "tls-example-*")
	if err != nil {
		fmt.Println("temp dir:", err)

		return
	}
	defer func() { _ = os.RemoveAll(dir) }()

	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	if _, _, err := diotls.CreateAndSaveSelfSignedKeyPair(selfSigned, certFile, keyFile); err != nil {
		fmt.Println("key pair:", err)

		return
	}

	serverTLS, err := diotls.NewServerTLSConfig(context.Background(), diotls.ServerConfig{
		LocalConfig: diotls.LocalConfig{Certificate: certFile, Key: keyFile},
	})
	if err != nil {
		fmt.Println("server tls config:", err)

		return
	}
	clientTLS, err := diotls.NewClientTLSConfig(diotls.ClientConfig{RootCAFile: certFile})
	if err != nil {
		fmt.Println("client tls config:", err)

		return
	}

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "hello over TLS")
	}))
	server.TLS = serverTLS
	server.StartTLS()
	defer server.Close()

	client := &http.Client{Transport: &http.Transport{TLSClientConfig: clientTLS}}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		fmt.Println("request:", err)

		return
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("get:", err)

		return
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)

	fmt.Println(resp.StatusCode, string(body))
	// Output:
	// 200 hello over TLS
}
