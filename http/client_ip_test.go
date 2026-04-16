package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.100, 10.0.0.1, 172.16.0.1")

	ip := GetClientIP(req)
	assert.Equal(t, "192.168.1.100", ip)
}

func TestGetClientIP_XForwardedFor_Single(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.100")

	ip := GetClientIP(req)
	assert.Equal(t, "192.168.1.100", ip)
}

func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Real-IP", "192.168.1.50")

	ip := GetClientIP(req)
	assert.Equal(t, "192.168.1.50", ip)
}

func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "10.0.0.5:12345"

	ip := GetClientIP(req)
	assert.Equal(t, "10.0.0.5", ip)
}

func TestGetClientIP_RemoteAddr_IPv6(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "[::1]:12345"

	ip := GetClientIP(req)
	assert.Equal(t, "::1", ip)
}

func TestGetClientIP_Priority(t *testing.T) {
	// X-Forwarded-For takes priority over X-Real-IP
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.100")
	req.Header.Set("X-Real-IP", "192.168.1.50")
	req.RemoteAddr = "10.0.0.5:12345"

	ip := GetClientIP(req)
	assert.Equal(t, "192.168.1.100", ip)
}

func TestGetClientIP_XRealIP_OverRemoteAddr(t *testing.T) {
	// X-Real-IP takes priority over RemoteAddr
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Real-IP", "192.168.1.50")
	req.RemoteAddr = "10.0.0.5:12345"

	ip := GetClientIP(req)
	assert.Equal(t, "192.168.1.50", ip)
}

func TestGetClientIP_Forwarded(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "Single for",
			header:   "for=192.0.2.60",
			expected: "192.0.2.60",
		},
		{
			name:     "Multiple params",
			header:   "for=192.0.2.60;proto=http;by=203.0.113.43",
			expected: "192.0.2.60",
		},
		{
			name:     "Multiple values",
			header:   "for=192.0.2.43, for=198.51.100.17",
			expected: "192.0.2.43",
		},
		{
			name:     "Quoted IPv6",
			header:   `for="[2001:db8:cafe::17]"`,
			expected: "2001:db8:cafe::17",
		},
		{
			name:     "Mixed case and spaces",
			header:   "For=192.0.2.60 ; Proto=https",
			expected: "192.0.2.60",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Forwarded", tt.header)
			ip := GetClientIP(req)
			assert.Equal(t, tt.expected, ip)
		})
	}
}
