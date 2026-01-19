package ip

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dioad/net/authz"
)

func TestHandlerFunc(t *testing.T) {
	cfg := authz.NetworkACLConfig{
		AllowedNets:    []string{"127.0.0.1/32"},
		AllowByDefault: false,
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	handlerFunc := HandlerFunc(cfg, nextHandler)

	// Test allowed IP
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()

	handlerFunc(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}
	if w.Body.String() != "success" {
		t.Errorf("Expected body %q, got %q", "success", w.Body.String())
	}
}

func TestNewHandler(t *testing.T) {
	cfg := authz.NetworkACLConfig{
		AllowedNets:    []string{"10.0.0.0/8"},
		AllowByDefault: false,
	}

	handler := NewHandler(cfg)

	if handler == nil {
		t.Fatal("Expected handler to be created, got nil")
	}
	if handler.Authoriser == nil {
		t.Fatal("Expected Authoriser to be set, got nil")
	}
}

func TestAuthRequest_Allowed(t *testing.T) {
	cfg := authz.NetworkACLConfig{
		AllowedNets:    []string{"192.168.0.0/16"},
		AllowByDefault: false,
	}

	handler := NewHandler(cfg)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:8080"

	ctx, err := handler.AuthRequest(req)

	if err != nil {
		t.Errorf("Expected no error for allowed IP, got: %v", err)
	}
	if ctx == nil {
		t.Error("Expected context to be returned")
	}
}

func TestAuthRequest_Denied(t *testing.T) {
	cfg := authz.NetworkACLConfig{
		AllowedNets:    []string{"10.0.0.0/8"},
		AllowByDefault: false,
	}

	handler := NewHandler(cfg)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	_, err := handler.AuthRequest(req)

	if err == nil {
		t.Error("Expected error for denied IP, got nil")
	}
}

func TestAuthRequest_InvalidAddress(t *testing.T) {
	cfg := authz.NetworkACLConfig{
		AllowedNets:    []string{"10.0.0.0/8"},
		AllowByDefault: false,
	}

	handler := NewHandler(cfg)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "invalid-address"

	_, err := handler.AuthRequest(req)

	if err == nil {
		t.Error("Expected error for invalid address, got nil")
	}
}

func TestWrap_Allowed(t *testing.T) {
	cfg := authz.NetworkACLConfig{
		AllowedNets:    []string{"172.16.0.0/12"},
		AllowByDefault: false,
	}

	handler := NewHandler(cfg)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("allowed"))
	})

	wrappedHandler := handler.Wrap(nextHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "172.16.100.50:9000"
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}
	if w.Body.String() != "allowed" {
		t.Errorf("Expected body %q, got %q", "allowed", w.Body.String())
	}
}

func TestWrap_Forbidden(t *testing.T) {
	cfg := authz.NetworkACLConfig{
		AllowedNets:    []string{"10.0.0.0/8"},
		AllowByDefault: false,
	}

	handler := NewHandler(cfg)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Next handler should not be called for forbidden request")
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := handler.Wrap(nextHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "203.0.113.100:1234"
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status code %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestWrap_AllowByDefault(t *testing.T) {
	cfg := authz.NetworkACLConfig{
		DeniedNets:     []string{"10.0.0.0/8"},
		AllowByDefault: true,
	}

	handler := NewHandler(cfg)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("allowed by default"))
	})

	wrappedHandler := handler.Wrap(nextHandler)

	// Test allowed IP (not in deny list)
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:8080"
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}
	if w.Body.String() != "allowed by default" {
		t.Errorf("Expected body %q, got %q", "allowed by default", w.Body.String())
	}
}
