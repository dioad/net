package hmac

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authcontext "github.com/dioad/net/http/auth/context"
)

func TestClientHandlerIntegration(t *testing.T) {
	const sharedKey = "super-secret-key"
	const principalID = "user123"
	const requestBody = `{"action": "create"}`

	serverHandler := NewHandler(ServerConfig{
		CommonConfig: CommonConfig{
			SharedKey: sharedKey,
		},
	})

	testServer := httptest.NewServer(
		serverHandler.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal := authcontext.AuthenticatedPrincipalFromContext(r.Context())
			if principal != principalID {
				t.Errorf("expected principal %q, got %q", principalID, principal)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("authenticated"))
		})),
	)
	defer testServer.Close()

	clientAuth := ClientAuth{
		Config: ClientConfig{
			CommonConfig: CommonConfig{
				SharedKey: sharedKey,
			},
			Principal: principalID,
		},
	}

	req, err := http.NewRequest("POST", testServer.URL+"/action", bytes.NewBufferString(requestBody))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	if err := clientAuth.AddAuth(req); err != nil {
		t.Fatalf("failed to add auth: %v", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestClientHandlerWithSignedHeaders(t *testing.T) {
	const sharedKey = "secret"
	const principalID = "user1"
	const customHeader = "X-Custom-Value"
	const customValue = "foobar"

	serverHandler := NewHandler(ServerConfig{
		CommonConfig: CommonConfig{
			SharedKey:     sharedKey,
			SignedHeaders: []string{customHeader},
		},
	})

	testServer := httptest.NewServer(
		serverHandler.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})),
	)
	defer testServer.Close()

	clientAuth := ClientAuth{
		Config: ClientConfig{
			CommonConfig: CommonConfig{
				SharedKey:     sharedKey,
				SignedHeaders: []string{customHeader},
			},
			Principal: principalID,
		},
	}

	req, err := http.NewRequest("GET", testServer.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set(customHeader, customValue)

	if err := clientAuth.AddAuth(req); err != nil {
		t.Fatalf("AddAuth failed: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Now try with modified header value
	req2, err := http.NewRequest("GET", testServer.URL, nil)
	if err != nil {
		t.Fatalf("failed to create tampered request: %v", err)
	}
	req2.Header.Set(customHeader, "WRONG")
	// Manually copy the auth headers from previous request to simulate tampering
	req2.Header.Set("Authorization", req.Header.Get("Authorization"))
	req2.Header.Set(DefaultTimestampHeader, req.Header.Get(DefaultTimestampHeader))
	req2.Header.Set(DefaultSignedHeadersHeader, req.Header.Get(DefaultSignedHeadersHeader))

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("failed to make tampered request: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for tampered header, got %d", resp2.StatusCode)
	}
}

func TestTimestampExpiry(t *testing.T) {
	const sharedKey = "secret"
	serverHandler := NewHandler(ServerConfig{
		CommonConfig: CommonConfig{
			SharedKey:        sharedKey,
			MaxTimestampDiff: 1 * time.Second,
		},
	})

	testServer := httptest.NewServer(serverHandler.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))
	defer testServer.Close()

	clientAuth := ClientAuth{
		Config: ClientConfig{
			CommonConfig: CommonConfig{SharedKey: sharedKey},
			Principal:    "user",
		},
	}

	req, _ := http.NewRequest("GET", testServer.URL, nil)
	clientAuth.AddAuth(req)

	// Wait for expiry
	time.Sleep(2 * time.Second)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired timestamp, got %d", resp.StatusCode)
	}
}

func TestWrongPathOrMethod(t *testing.T) {
	const sharedKey = "secret"
	serverHandler := NewHandler(ServerConfig{
		CommonConfig: CommonConfig{SharedKey: sharedKey},
	})

	testServer := httptest.NewServer(serverHandler.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))
	defer testServer.Close()

	clientAuth := ClientAuth{
		Config: ClientConfig{
			CommonConfig: CommonConfig{SharedKey: sharedKey},
			Principal:    "user",
		},
	}

	req, _ := http.NewRequest("GET", testServer.URL+"/valid", nil)
	clientAuth.AddAuth(req)

	// Change path manually
	req.URL.Path = "/invalid"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong path, got %d", resp.StatusCode)
	}
}

func TestPrincipalSpoofing(t *testing.T) {
	const sharedKey = "super-secret-key"
	const userPrincipal = "user123"
	const adminPrincipal = "admin"

	serverHandler := NewHandler(ServerConfig{
		CommonConfig: CommonConfig{SharedKey: sharedKey},
	})

	testServer := httptest.NewServer(serverHandler.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))
	defer testServer.Close()

	clientAuth := ClientAuth{Config: ClientConfig{
		CommonConfig: CommonConfig{SharedKey: sharedKey},
		Principal:    userPrincipal,
	}}

	req1, _ := http.NewRequest("POST", testServer.URL, bytes.NewBufferString("body"))
	clientAuth.AddAuth(req1)

	// Capture the valid token for userPrincipal
	authHeader := req1.Header.Get("Authorization")

	// Attacker reuses the signature but changes the principal in the Authorization header
	// Authorization: HMAC user123:signature -> Authorization: HMAC admin:signature
	parts := strings.Split(authHeader, " ")
	creds := parts[1]
	signature := strings.Split(creds, ":")[1]
	spoofedAuthHeader := fmt.Sprintf("HMAC %s:%s", adminPrincipal, signature)

	req2, _ := http.NewRequest("POST", testServer.URL, bytes.NewBufferString("body"))
	req2.Header.Set("Authorization", spoofedAuthHeader)
	req2.Header.Set(DefaultTimestampHeader, req1.Header.Get(DefaultTimestampHeader))

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for spoofed principal, got %d", resp2.StatusCode)
	}
}

func TestHMACRoundTripper(t *testing.T) {
	const sharedKey = "secret"
	const principalID = "round-tripper-user"

	serverHandler := NewHandler(ServerConfig{
		CommonConfig: CommonConfig{SharedKey: sharedKey},
	})

	testServer := httptest.NewServer(serverHandler.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	defer testServer.Close()

	client := &http.Client{
		Transport: &HMACRoundTripper{
			Config: ClientConfig{
				CommonConfig: CommonConfig{SharedKey: sharedKey},
				Principal:    principalID,
			},
		},
	}

	resp, err := client.Get(testServer.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func BenchmarkClientAddAuth(b *testing.B) {
	clientAuth := ClientAuth{
		Config: ClientConfig{
			CommonConfig: CommonConfig{
				SharedKey:     "bench-key",
				SignedHeaders: []string{"Content-Type", "X-Custom"},
			},
			Principal: "bench-user",
		},
	}
	req, _ := http.NewRequest("POST", "http://example.com/api", bytes.NewBufferString(`{"data": true}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Custom", "value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = clientAuth.AddAuth(req)
	}
}
