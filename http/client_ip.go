package http

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"
)

// httpContextKeyClientIP is an unexported type used as a key for storing the client IP in the context.
type httpContextKeyClientIP struct{}

// ContextWithClientIP extracts the client IP address from the request and stores it in the context.
// It checks X-Forwarded-For and X-Real-IP headers first (for proxied requests),
// then falls back to RemoteAddr.
func ContextWithClientIP(ctx context.Context, r *http.Request) context.Context {
	ip := GetClientIP(r)

	return context.WithValue(ctx, httpContextKeyClientIP{}, ip)
}

// ClientIPFromContext retrieves the client IP address from the context.
func ClientIPFromContext(ctx context.Context) (string, bool) {
	ip, ok := ctx.Value(httpContextKeyClientIP{}).(string)
	return ip, ok
}

// GetClientIP extracts the client IP address from a request.
// It checks X-Forwarded-For and X-Real-IP headers first (for proxied requests),
// then falls back to RemoteAddr.
//
// X-Forwarded-For, Forwarded, and X-Real-IP are ordinary request headers:
// any client can set them to an arbitrary value, and this function trusts
// them unconditionally with no allowlist of known proxy addresses. Unless
// the caller has independently verified that the request reached this
// process through a proxy it controls, which strips or overwrites these
// headers before forwarding, the returned value must be treated as
// attacker-controlled input -- not used as an identity or trust boundary
// for rate limiting, access control, or audit logging without that
// guarantee in place.
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (may contain multiple IPs)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list (original client)
		if before, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(before)
		}
		return strings.TrimSpace(xff)
	}

	// Check Forwarded header
	if f := r.Header.Get("Forwarded"); f != "" {
		// Take the first value (original client)
		return parseForwardedHeader(f)
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr (remove port if present)
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		// Handle IPv6 addresses like [::1]:8080
		if strings.HasPrefix(addr, "[") {
			if bracketIdx := strings.Index(addr, "]"); bracketIdx != -1 {
				return addr[1:bracketIdx]
			}
		}
		return addr[:idx]
	}
	return addr
}

func parseForwardedHeader(f string) string {
	if first, _, ok := strings.Cut(f, ","); ok {
		f = first
	}
	// Look for for=
	for part := range strings.SplitSeq(f, ";") {
		part = strings.TrimSpace(part)
		if before, after, ok := strings.Cut(part, "="); ok {
			if strings.EqualFold(before, "for") {
				ip := strings.TrimSpace(after)
				// Remove quotes if present
				ip = strings.Trim(ip, "\"")
				// Remove brackets if present (IPv6)
				ip = strings.TrimPrefix(ip, "[")
				ip = strings.TrimSuffix(ip, "]")
				return ip
			}
		}
	}
	return ""
}

// ClientIPResolver resolves a request's client IP address under an
// explicit trust model, unlike the package-level GetClientIP, which trusts
// forwarding headers unconditionally (see its doc comment). Zero or one
// trust mode may be configured:
//
//   - WithTrustedHeader trusts one named header verbatim, for platforms
//     whose edge sets a header this process can only receive by going
//     through that edge (e.g. fly.io's Fly-Client-IP -- see
//     NewFlyClientIPResolver).
//   - WithTrustedProxyCIDRs walks X-Forwarded-For from the right against a
//     caller-supplied set of trusted proxy ranges, for platforms that
//     publish stable ranges (AWS ALB, Azure Application Gateway,
//     Cloudflare, self-hosted nginx, etc).
//
// If both are configured, WithTrustedHeader takes precedence. If neither
// is configured, ClientIP falls back to the request's direct peer address
// (RemoteAddr) only -- it never trusts a client-supplied header, unlike
// GetClientIP.
type ClientIPResolver struct {
	trustedHeader     string
	trustedProxyCIDRs []netip.Prefix
}

// ClientIPResolverOption configures a ClientIPResolver.
type ClientIPResolverOption func(*ClientIPResolver)

// WithTrustedHeader configures the resolver to trust one named header
// verbatim. Use this only when the deployment topology guarantees this
// process receives no traffic that hasn't passed through the edge that
// sets this header -- there is no source-address check involved.
func WithTrustedHeader(name string) ClientIPResolverOption {
	return func(c *ClientIPResolver) { c.trustedHeader = name }
}

// WithTrustedProxyCIDRs configures the resolver to walk X-Forwarded-For
// from the right, skipping entries whose address falls within the given
// CIDR ranges, and returning the first entry that doesn't -- the RFC
// 7239-style trusted-proxy-list algorithm. The request's direct peer
// (RemoteAddr) must also fall within the trusted set for X-Forwarded-For
// to be consulted at all; otherwise the peer is untrusted and is used
// directly, since any header it presents is unverified.
func WithTrustedProxyCIDRs(cidrs ...netip.Prefix) ClientIPResolverOption {
	return func(c *ClientIPResolver) { c.trustedProxyCIDRs = append(c.trustedProxyCIDRs, cidrs...) }
}

// NewClientIPResolver creates a ClientIPResolver with the given options.
func NewClientIPResolver(opts ...ClientIPResolverOption) *ClientIPResolver {
	c := &ClientIPResolver{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// NewFlyClientIPResolver returns a ClientIPResolver configured to trust
// Fly Proxy's Fly-Client-IP header, which reflects Fly Proxy's own direct
// observation of the TCP peer it accepted a connection from -- not a value
// copied from client input, so there is nothing for a client to spoof.
// This is correct as long as this process is only reachable by going
// through fly-proxy (the normal fly.io deployment shape; Fly Machines
// aren't directly Internet-routable otherwise). If another CDN or proxy is
// placed in front of fly.io, Fly-Client-IP reflects that intermediate hop
// instead of the true client -- use WithTrustedProxyCIDRs with that
// layer's published ranges instead.
func NewFlyClientIPResolver() *ClientIPResolver {
	return NewClientIPResolver(WithTrustedHeader("Fly-Client-IP"))
}

// ClientIP resolves r's client IP under the configured trust mode.
func (c *ClientIPResolver) ClientIP(r *http.Request) string {
	if c.trustedHeader != "" {
		return c.clientIPFromTrustedHeader(r)
	}
	if len(c.trustedProxyCIDRs) > 0 {
		return c.clientIPFromTrustedProxyCIDRs(r)
	}
	return c.remoteAddrString(r)
}

// PrincipalFunc adapts ClientIP to the PrincipalFunc signature, for use
// with WithRateLimiterPrincipalFunc/WithPrincipalFunc.
func (c *ClientIPResolver) PrincipalFunc(r *http.Request) (string, error) {
	return c.ClientIP(r), nil
}

func (c *ClientIPResolver) clientIPFromTrustedHeader(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get(c.trustedHeader)); v != "" {
		return v
	}
	// The trusted header is unexpectedly absent (e.g. something hit this
	// process directly, bypassing the edge that normally sets it).
	return c.remoteAddrString(r)
}

func (c *ClientIPResolver) clientIPFromTrustedProxyCIDRs(r *http.Request) string {
	peer, ok := remoteAddrIP(r)
	if !ok || !c.isTrustedProxy(peer) {
		// The immediate peer isn't a trusted proxy (or couldn't be
		// parsed), so any X-Forwarded-For it presents is unverified --
		// the peer itself is the only address that can be trusted.
		return c.remoteAddrString(r)
	}

	for entry := range reverseSplitSeq(r.Header.Get("X-Forwarded-For"), ",") {
		addr, ok := parseForwardedIP(entry)
		if !ok {
			continue
		}
		if !c.isTrustedProxy(addr) {
			return addr.String()
		}
	}

	// Every entry (or none at all) was a trusted proxy; fall back to the peer.
	return peer.String()
}

func (c *ClientIPResolver) isTrustedProxy(addr netip.Addr) bool {
	for _, cidr := range c.trustedProxyCIDRs {
		if cidr.Contains(addr) {
			return true
		}
	}
	return false
}

// remoteAddrString returns r.RemoteAddr's IP, or the raw RemoteAddr if it
// can't be parsed.
func (c *ClientIPResolver) remoteAddrString(r *http.Request) string {
	if peer, ok := remoteAddrIP(r); ok {
		return peer.String()
	}
	return r.RemoteAddr
}

// remoteAddrIP extracts and parses the IP portion of r.RemoteAddr, which
// net/http guarantees to be in "IP:port" form.
func remoteAddrIP(r *http.Request) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(host)
	return addr, err == nil
}

// parseForwardedIP parses a single X-Forwarded-For entry, tolerating
// surrounding whitespace and bracketed IPv6 literals.
func parseForwardedIP(s string) (netip.Addr, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	addr, err := netip.ParseAddr(s)
	return addr, err == nil
}

// reverseSplitSeq splits s on sep and yields the parts from right to left.
func reverseSplitSeq(s, sep string) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		if s == "" {
			return
		}
		parts := strings.Split(s, sep)
		for _, part := range slices.Backward(parts) {
			if !yield(part) {
				return
			}
		}
	}
}
