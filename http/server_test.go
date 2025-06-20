package http

import (
	"context"

	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	dnt "github.com/dioad/net/tls"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"golang.org/x/net/nettest"
)

func TestNewDefaultServer(t *testing.T) {
	c := Config{}

	s := newDefaultServer(c)

	ln, _ := nettest.NewLocalListener("tcp4")

	go func() {
		s.Serve(ln)
	}()

	err := s.Shutdown(context.Background())
	if err != nil {
		t.Error(err)
	}
}

func TestMultipleNewDefaultServer(t *testing.T) {
	c := Config{}

	s1 := newDefaultServer(c)
	s2 := newDefaultServer(c)

	ln1, _ := nettest.NewLocalListener("tcp4")
	ln2, _ := nettest.NewLocalListener("tcp4")

	go func() {
		s1.Serve(ln1)
		s2.Serve(ln2)
	}()

	err := s1.Shutdown(context.Background())
	if err != nil {
		t.Error(err)
	}

	err = s2.Shutdown(context.Background())
	if err != nil {
		t.Error(err)
	}
}

// TestServerWithOptions tests creating a server with various options
func TestServerWithOptions(t *testing.T) {
	// Create a server with all options enabled
	config := Config{
		ListenAddress:           ":0", // Use a random port
		EnablePrometheusMetrics: true,
		EnableDebug:             true,
		EnableStatus:            true,
		EnableProxyProtocol:     false,
	}

	// Create a logger that discards output for testing
	logger := zerolog.New(io.Discard).With().Timestamp().Logger()

	// Create the server with options
	server := NewServer(
		config,
		WithLogger(logger),
	)

	// Start the server
	ln, err := nettest.NewLocalListener("tcp4")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			t.Errorf("Server error: %v", err)
		}
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Errorf("Failed to shutdown server: %v", err)
	}
}

// TestServerWithTLS tests creating a server with TLS configuration
func TestServerWithTLS(t *testing.T) {
	config := Config{
		ListenAddress: ":0", // Use a random port
	}

	tlsConfig, _ := dnt.NewServerTLSConfig(context.Background(), dnt.ServerConfig{
		SelfSignedConfig: dnt.SelfSignedConfig{
			CacheDirectory: t.TempDir(),
			Subject: dnt.CertificateSubject{
				CommonName:   "TestServerWithTLS",
				Organization: []string{"TestOrg"},
				Country:      []string{"GB"},
			},
			SANConfig: dnt.SANConfig{
				DNSNames:    []string{"localhost"},
				IPAddresses: []string{"127.0.0.1"},
			},
			Duration: "5m",
		},
	})

	// Create a minimal TLS config for testing
	// tlsConfig := &tls.Config{
	//
	// 	InsecureSkipVerify: true,
	// }

	server := NewServer(config)
	server.Config.TLSConfig = tlsConfig

	// Start the server with TLS
	ln, err := nettest.NewLocalListener("tcp4")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			t.Errorf("Server error: %v", err)
		}
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Errorf("Failed to shutdown server: %v", err)
	}
}

// MockResource implements DefaultResource for testing
type MockResource struct {
	RegisterRoutesCalled bool
}

func (m *MockResource) RegisterRoutes(router *mux.Router) {
	m.RegisterRoutesCalled = true
	router.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	})
}

// TestAddResource tests adding a resource to the server
func TestAddResource(t *testing.T) {
	server := NewServer(Config{})
	mockResource := &MockResource{}

	// Add the resource
	server.AddResource("/api", mockResource)

	// Check that the resource was added
	if !mockResource.RegisterRoutesCalled {
		t.Error("RegisterRoutes was not called")
	}

	// Check that the resource is in the resource map
	if _, ok := server.ResourceMap["/api"]; !ok {
		t.Error("Resource was not added to the resource map")
	}

	// Create a test request
	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	// Serve the request
	server.handler().ServeHTTP(w, req)

	// Check the response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}
	if w.Body.String() != "test" {
		t.Errorf("Expected body %q, got %q", "test", w.Body.String())
	}
}

// MockStatusResource implements StatusResource for testing
type MockStatusResource struct {
	MockResource
	StatusCalled bool
	StatusError  bool
}

func (m *MockStatusResource) Status() (interface{}, error) {
	m.StatusCalled = true
	if m.StatusError {
		return nil, io.ErrUnexpectedEOF
	}
	return map[string]string{"status": "ok"}, nil
}

// TestStatusEndpoint tests the status endpoint
func TestStatusEndpoint(t *testing.T) {
	config := Config{
		EnableStatus: true,
	}
	server := NewServer(config)
	mockResource := &MockStatusResource{}

	// Add the resource
	server.AddResource("/api", mockResource)

	// Add a static metadata item
	server.AddStatusStaticMetadataItem("version", "1.0.0")

	// Create a test request
	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()

	// Serve the request
	server.handler().ServeHTTP(w, req)

	// Check the response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	// Parse the response
	var statusResponse map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &statusResponse); err != nil {
		t.Fatalf("Failed to parse status response: %v", err)
	}

	// Check that the resource status is included
	routes, ok := statusResponse["Routes"].(map[string]interface{})
	if !ok {
		t.Error("Routes not found in status response")
	} else {
		apiStatus, ok := routes["/api"].(map[string]interface{})
		if !ok {
			t.Error("API status not found in routes")
		} else {
			if apiStatus["status"] != "ok" {
				t.Errorf("Expected status %q, got %q", "ok", apiStatus["status"])
			}
		}
	}

	// Check that the metadata is included
	metadata, ok := statusResponse["Metadata"].(map[string]interface{})
	if !ok {
		t.Error("Metadata not found in status response")
	} else {
		if metadata["version"] != "1.0.0" {
			t.Errorf("Expected version %q, got %q", "1.0.0", metadata["version"])
		}
	}
}

// TestMiddleware tests adding middleware to the server
func TestMiddleware(t *testing.T) {
	server := NewServer(Config{})

	// Add a middleware that adds a header
	server.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test", "test")
			next.ServeHTTP(w, r)
		})
	})

	// Add a handler
	server.AddHandlerFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	})

	// Create a test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Serve the request
	server.handler().ServeHTTP(w, req)

	// Check the response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}
	if w.Header().Get("X-Test") != "test" {
		t.Errorf("Expected header %q, got %q", "test", w.Header().Get("X-Test"))
	}
}

// mockAuthMiddleware is a simple implementation of auth.Middleware for testing
type mockAuthMiddleware struct {
	handler http.Handler
}

func (m *mockAuthMiddleware) Wrap(_ http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	})
}
