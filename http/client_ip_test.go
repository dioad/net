package http

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestClientIPResolver_TrustedHeader(t *testing.T) {
	resolver := NewClientIPResolver(WithTrustedHeader("Fly-Client-IP"))

	t.Run("uses the trusted header when present", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Fly-Client-IP", "203.0.113.5")
		req.Header.Set("X-Forwarded-For", "attacker-controlled")
		req.RemoteAddr = "10.0.0.5:12345"

		assert.Equal(t, "203.0.113.5", resolver.ClientIP(req))
	})

	t.Run("falls back to RemoteAddr when the trusted header is absent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "10.0.0.5:12345"

		assert.Equal(t, "10.0.0.5", resolver.ClientIP(req))
	})
}

func TestNewFlyClientIPResolver(t *testing.T) {
	resolver := NewFlyClientIPResolver()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Fly-Client-IP", "203.0.113.7")

	assert.Equal(t, "203.0.113.7", resolver.ClientIP(req))
}

func mustPrefix(t *testing.T, s string) netip.Prefix {
	t.Helper()
	p, err := netip.ParsePrefix(s)
	require.NoError(t, err)
	return p
}

func TestClientIPResolver_TrustedProxyCIDRs(t *testing.T) {
	resolver := NewClientIPResolver(WithTrustedProxyCIDRs(mustPrefix(t, "10.0.0.0/8")))

	t.Run("untrusted peer: XFF is ignored, the peer itself is the client", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "1.2.3.4")
		req.RemoteAddr = "203.0.113.9:12345" // not in 10.0.0.0/8

		assert.Equal(t, "203.0.113.9", resolver.ClientIP(req), "a peer outside the trusted set must not have its X-Forwarded-For trusted")
	})

	t.Run("trusted peer: walks XFF from the right, returns the first untrusted entry", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		// leftmost is client-claimed (untrusted-looking but happens to be
		// the real client here); rightmost is the trusted proxy that
		// actually appended it.
		req.Header.Set("X-Forwarded-For", "198.51.100.1, 10.0.0.2")
		req.RemoteAddr = "10.0.0.2:12345" // in 10.0.0.0/8: trusted

		assert.Equal(t, "198.51.100.1", resolver.ClientIP(req))
	})

	t.Run("trusted peer with an entirely trusted XFF chain falls back to the peer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.3, 10.0.0.2")
		req.RemoteAddr = "10.0.0.2:12345"

		assert.Equal(t, "10.0.0.2", resolver.ClientIP(req))
	})

	t.Run("trusted peer with no XFF header falls back to the peer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "10.0.0.2:12345"

		assert.Equal(t, "10.0.0.2", resolver.ClientIP(req))
	})

	t.Run("skips a malformed XFF entry and continues walking left", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "198.51.100.1, not-an-ip, 10.0.0.2")
		req.RemoteAddr = "10.0.0.2:12345"

		assert.Equal(t, "198.51.100.1", resolver.ClientIP(req))
	})
}

func TestClientIPResolver_NoTrustModeConfigured(t *testing.T) {
	resolver := NewClientIPResolver()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "attacker-controlled")
	req.RemoteAddr = "10.0.0.5:12345"

	assert.Equal(t, "10.0.0.5", resolver.ClientIP(req), "with no trust mode configured, only RemoteAddr is used -- never a client-supplied header")
}

func TestClientIPResolver_PrincipalFunc(t *testing.T) {
	resolver := NewClientIPResolver(WithTrustedHeader("Fly-Client-IP"))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Fly-Client-IP", "203.0.113.5")

	principal, err := resolver.PrincipalFunc(req)
	require.NoError(t, err)
	assert.Equal(t, "203.0.113.5", principal)
}
