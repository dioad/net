package http

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/dioad/config/configcheck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.100, 10.0.0.1, 172.16.0.1")

	ip := GetClientIP(req)
	assert.Equal(t, "192.168.1.100", ip)
}

func TestGetClientIP_XForwardedFor_Single(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.100")

	ip := GetClientIP(req)
	assert.Equal(t, "192.168.1.100", ip)
}

func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.Header.Set("X-Real-IP", "192.168.1.50")

	ip := GetClientIP(req)
	assert.Equal(t, "192.168.1.50", ip)
}

func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.RemoteAddr = "10.0.0.5:12345"

	ip := GetClientIP(req)
	assert.Equal(t, "10.0.0.5", ip)
}

func TestGetClientIP_RemoteAddr_IPv6(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.RemoteAddr = "[::1]:12345"

	ip := GetClientIP(req)
	assert.Equal(t, "::1", ip)
}

func TestGetClientIP_Priority(t *testing.T) {
	// X-Forwarded-For takes priority over X-Real-IP
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.100")
	req.Header.Set("X-Real-IP", "192.168.1.50")
	req.RemoteAddr = "10.0.0.5:12345"

	ip := GetClientIP(req)
	assert.Equal(t, "192.168.1.100", ip)
}

func TestGetClientIP_XRealIP_OverRemoteAddr(t *testing.T) {
	// X-Real-IP takes priority over RemoteAddr
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
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
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
			req.Header.Set("Forwarded", tt.header)
			ip := GetClientIP(req)
			assert.Equal(t, tt.expected, ip)
		})
	}
}

func TestClientIPResolver_TrustedHeader(t *testing.T) {
	resolver := NewClientIPResolver(WithTrustedHeader("Fly-Client-IP"))

	t.Run("uses the trusted header when present", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
		req.Header.Set("Fly-Client-Ip", "203.0.113.5")
		req.Header.Set("X-Forwarded-For", "attacker-controlled")
		req.RemoteAddr = "10.0.0.5:12345"

		assert.Equal(t, "203.0.113.5", resolver.ClientIP(req))
	})

	t.Run("falls back to RemoteAddr when the trusted header is absent", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
		req.RemoteAddr = "10.0.0.5:12345"

		assert.Equal(t, "10.0.0.5", resolver.ClientIP(req))
	})
}

func TestNewFlyClientIPResolver(t *testing.T) {
	resolver := NewFlyClientIPResolver()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.Header.Set("Fly-Client-Ip", "203.0.113.7")

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
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "1.2.3.4")
		req.RemoteAddr = "203.0.113.9:12345" // not in 10.0.0.0/8

		assert.Equal(t, "203.0.113.9", resolver.ClientIP(req), "a peer outside the trusted set must not have its X-Forwarded-For trusted")
	})

	t.Run("trusted peer: walks XFF from the right, returns the first untrusted entry", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
		// leftmost is client-claimed (untrusted-looking but happens to be
		// the real client here); rightmost is the trusted proxy that
		// actually appended it.
		req.Header.Set("X-Forwarded-For", "198.51.100.1, 10.0.0.2")
		req.RemoteAddr = "10.0.0.2:12345" // in 10.0.0.0/8: trusted

		assert.Equal(t, "198.51.100.1", resolver.ClientIP(req))
	})

	t.Run("trusted peer with an entirely trusted XFF chain falls back to the peer", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.3, 10.0.0.2")
		req.RemoteAddr = "10.0.0.2:12345"

		assert.Equal(t, "10.0.0.2", resolver.ClientIP(req))
	})

	t.Run("trusted peer with no XFF header falls back to the peer", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
		req.RemoteAddr = "10.0.0.2:12345"

		assert.Equal(t, "10.0.0.2", resolver.ClientIP(req))
	})

	t.Run("a malformed XFF entry ends the walk at the nearest trusted hop", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "198.51.100.1, not-an-ip, 10.0.0.3")
		req.RemoteAddr = "10.0.0.2:12345"

		assert.Equal(t, "10.0.0.3", resolver.ClientIP(req), "an address left of a malformed hop was not vouched for by a trusted proxy")
	})
}

func TestClientIPResolver_NoTrustModeConfigured(t *testing.T) {
	resolver := NewClientIPResolver()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "attacker-controlled")
	req.RemoteAddr = "10.0.0.5:12345"

	assert.Equal(t, "10.0.0.5", resolver.ClientIP(req), "with no trust mode configured, only RemoteAddr is used -- never a client-supplied header")
}

func TestClientIPResolver_PrincipalFunc(t *testing.T) {
	resolver := NewClientIPResolver(WithTrustedHeader("Fly-Client-IP"))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.Header.Set("Fly-Client-Ip", "203.0.113.5")

	principal, err := resolver.PrincipalFunc(req)
	require.NoError(t, err)
	assert.Equal(t, "203.0.113.5", principal)
}

func clientAddrRequest(t *testing.T, remote string, headers map[string][]string) *http.Request {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.RemoteAddr = remote
	for k, vs := range headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	return req
}

func TestClientIPResolver_ClientAddr_UntrustedPeerCannotChooseAddress(t *testing.T) {
	resolver := NewClientIPResolver(WithTrustedProxyCIDRs(mustPrefix(t, "10.0.0.0/8")))

	for _, header := range []string{"X-Forwarded-For", "Forwarded", "X-Real-IP"} {
		t.Run(header, func(t *testing.T) {
			value := "1.2.3.4"
			if header == "Forwarded" {
				value = "for=1.2.3.4"
			}
			req := clientAddrRequest(t, "203.0.113.9:4000", map[string][]string{header: {value}})

			assert.Equal(t, netip.MustParseAddr("203.0.113.9"), resolver.ClientAddr(req))
		})
	}
}

func TestClientIPResolver_ClientAddr_NoTrustMode(t *testing.T) {
	resolver := NewClientIPResolver()
	req := clientAddrRequest(t, "[2001:db8::7]:4000", map[string][]string{"X-Forwarded-For": {"1.2.3.4"}})

	assert.Equal(t, netip.MustParseAddr("2001:db8::7"), resolver.ClientAddr(req))
}

func TestClientIPResolver_ClientAddr_TrustedProxyChains(t *testing.T) {
	resolver := NewClientIPResolver(WithTrustedProxyCIDRs(mustPrefix(t, "10.0.0.0/8"), mustPrefix(t, "fd00::/8")))

	tests := []struct {
		name    string
		remote  string
		headers map[string][]string
		want    string
	}{
		{"multiple hops", "10.0.0.1:1", map[string][]string{"X-Forwarded-For": {"198.51.100.1, 10.0.0.3, 10.0.0.2"}}, "198.51.100.1"},
		{"client-forged left entries are ignored", "10.0.0.1:1", map[string][]string{"X-Forwarded-For": {"9.9.9.9, 198.51.100.1, 10.0.0.2"}}, "198.51.100.1"},
		{"repeated XFF header lines", "10.0.0.1:1", map[string][]string{"X-Forwarded-For": {"9.9.9.9", "198.51.100.1, 10.0.0.2"}}, "198.51.100.1"},
		{"IPv6 client", "10.0.0.1:1", map[string][]string{"X-Forwarded-For": {"2001:db8::5, 10.0.0.2"}}, "2001:db8::5"},
		{"IPv6 proxies", "[fd00::1]:1", map[string][]string{"X-Forwarded-For": {"2001:db8::5, fd00::2"}}, "2001:db8::5"},
		{"bracketed IPv6 with port", "10.0.0.1:1", map[string][]string{"X-Forwarded-For": {`[2001:db8::5]:4711, 10.0.0.2`}}, "2001:db8::5"},
		{"IPv4 with port", "10.0.0.1:1", map[string][]string{"X-Forwarded-For": {"198.51.100.1:5000"}}, "198.51.100.1"},
		{"IPv4-mapped IPv6", "10.0.0.1:1", map[string][]string{"X-Forwarded-For": {"::ffff:198.51.100.1"}}, "198.51.100.1"},
		{"Forwarded header", "10.0.0.1:1", map[string][]string{"Forwarded": {`for=198.51.100.1;proto=https, for=10.0.0.2`}}, "198.51.100.1"},
		{"Forwarded quoted IPv6", "10.0.0.1:1", map[string][]string{"Forwarded": {`for="[2001:db8::5]:4711", for=10.0.0.2`}}, "2001:db8::5"},
		{"Forwarded obfuscated identifier ends the walk", "10.0.0.1:1", map[string][]string{"Forwarded": {`for=_hidden, for=10.0.0.2`}}, "10.0.0.2"},
		{"XFF wins over Forwarded", "10.0.0.1:1", map[string][]string{"X-Forwarded-For": {"198.51.100.1"}, "Forwarded": {"for=198.51.100.2"}}, "198.51.100.1"},
		{"empty XFF", "10.0.0.1:1", map[string][]string{"X-Forwarded-For": {""}}, "10.0.0.1"},
		{"whitespace only XFF", "10.0.0.1:1", map[string][]string{"X-Forwarded-For": {" , "}}, "10.0.0.1"},
		{"only malformed entries", "10.0.0.1:1", map[string][]string{"X-Forwarded-For": {"garbage"}}, "10.0.0.1"},
		{"all hops trusted", "10.0.0.1:1", map[string][]string{"X-Forwarded-For": {"10.0.0.3, 10.0.0.2"}}, "10.0.0.1"},
		{"no headers", "10.0.0.1:1", nil, "10.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, netip.MustParseAddr(tt.want), resolver.ClientAddr(clientAddrRequest(t, tt.remote, tt.headers)))
		})
	}
}

func TestClientIPResolver_ClientAddr_TrustedHeader(t *testing.T) {
	resolver := NewFlyClientIPResolver()

	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"valid", "203.0.113.5", "203.0.113.5"},
		{"malformed falls back to the peer", "garbage", "10.0.0.5"},
		{"absent falls back to the peer", "", "10.0.0.5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string][]string{}
			if tt.value != "" {
				headers["Fly-Client-IP"] = []string{tt.value}
			}

			assert.Equal(t, netip.MustParseAddr(tt.want), resolver.ClientAddr(clientAddrRequest(t, "10.0.0.5:1", headers)))
		})
	}
}

func TestClientIPResolver_ClientAddr_UnparseableRemoteAddr(t *testing.T) {
	resolver := NewClientIPResolver(WithTrustedProxyCIDRs(mustPrefix(t, "10.0.0.0/8")))
	req := clientAddrRequest(t, "@", map[string][]string{"X-Forwarded-For": {"1.2.3.4"}})

	assert.False(t, resolver.ClientAddr(req).IsValid())
	assert.Equal(t, "@", resolver.ClientIP(req))
}

func TestClientIPConfig_Check(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ClientIPConfig
		wantErr bool
	}{
		{"empty", ClientIPConfig{}, false},
		{"valid proxies", ClientIPConfig{TrustedProxies: []string{"10.0.0.0/8", "fd00::/8"}}, false},
		{"header only", ClientIPConfig{TrustedHeader: "Fly-Client-IP"}, false},
		{"bad prefix", ClientIPConfig{TrustedProxies: []string{"10.0.0.0/8", "10.0.0.1"}}, true},
		{"both modes", ClientIPConfig{TrustedHeader: "Fly-Client-IP", TrustedProxies: []string{"10.0.0.0/8"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := configcheck.NewCollector()
			tt.cfg.Check(c)

			if tt.wantErr {
				require.Error(t, c.Err())
			} else {
				require.NoError(t, c.Err())
			}
		})
	}
}

func TestClientIPConfig_Resolver(t *testing.T) {
	resolver, err := ClientIPConfig{TrustedProxies: []string{"10.0.0.0/8"}}.Resolver()
	require.NoError(t, err)

	req := clientAddrRequest(t, "10.0.0.1:1", map[string][]string{"X-Forwarded-For": {"198.51.100.1"}})
	assert.Equal(t, netip.MustParseAddr("198.51.100.1"), resolver.ClientAddr(req))

	_, err = ClientIPConfig{TrustedProxies: []string{"nope"}}.Resolver()
	require.Error(t, err)
}
