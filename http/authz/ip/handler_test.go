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

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	handlerFunc, err := HandlerFunc(cfg, nextHandler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test allowed IP
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
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

	handler, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

	handler, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
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

	handler, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:8080"

	_, err = handler.AuthRequest(req)

	if err == nil {
		t.Error("Expected error for denied IP, got nil")
	}
}

func TestAuthRequest_InvalidAddress(t *testing.T) {
	cfg := authz.NetworkACLConfig{
		AllowedNets:    []string{"10.0.0.0/8"},
		AllowByDefault: false,
	}

	handler, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.RemoteAddr = "invalid-address"

	_, err = handler.AuthRequest(req)

	if err == nil {
		t.Error("Expected error for invalid address, got nil")
	}
}

func TestWrap(t *testing.T) {
	tests := []struct {
		name           string
		cfg            authz.NetworkACLConfig
		remoteAddr     string
		wantStatus     int
		wantBody       string
		wantNextCalled bool
	}{
		{
			name:           "allowed",
			cfg:            authz.NetworkACLConfig{AllowedNets: []string{"172.16.0.0/12"}, AllowByDefault: false},
			remoteAddr:     "172.16.100.50:9000",
			wantStatus:     http.StatusOK,
			wantBody:       "allowed",
			wantNextCalled: true,
		},
		{
			name:           "forbidden",
			cfg:            authz.NetworkACLConfig{AllowedNets: []string{"10.0.0.0/8"}, AllowByDefault: false},
			remoteAddr:     "203.0.113.100:1234",
			wantStatus:     http.StatusForbidden,
			wantNextCalled: false,
		},
		{
			name:           "allow by default (not in deny list)",
			cfg:            authz.NetworkACLConfig{DeniedNets: []string{"10.0.0.0/8"}, AllowByDefault: true},
			remoteAddr:     "192.168.1.1:8080",
			wantStatus:     http.StatusOK,
			wantBody:       "allowed by default",
			wantNextCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, err := NewHandler(tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			nextCalled := false
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tt.wantBody))
			})

			wrappedHandler := handler.Wrap(nextHandler)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			w := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(w, req)

			if nextCalled != tt.wantNextCalled {
				t.Errorf("next handler called = %v, want %v", nextCalled, tt.wantNextCalled)
			}
			if w.Code != tt.wantStatus {
				t.Errorf("Expected status code %d, got %d", tt.wantStatus, w.Code)
			}
			if tt.wantNextCalled && w.Body.String() != tt.wantBody {
				t.Errorf("Expected body %q, got %q", tt.wantBody, w.Body.String())
			}
		})
	}
}
