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
