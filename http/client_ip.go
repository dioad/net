package http

import (
	"context"
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
// It is suitable for display and logging only. For a rate-limit key, access
// control or any other trust decision use ClientIPResolver.ClientAddr, which
// ignores these headers unless the peer is a configured trusted proxy.
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
	if before, _, ok := strings.CutLast(addr, ":"); ok {
		// Handle IPv6 addresses like [::1]:8080
		if strings.HasPrefix(addr, "[") {
			if bracketIdx := strings.Index(addr, "]"); bracketIdx != -1 {
				return addr[1:bracketIdx]
			}
		}

		return before
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

// ClientIPResolver resolves a request's client address under an explicit
// trust model, unlike the package-level GetClientIP, which trusts forwarding
// headers unconditionally (see its doc comment). Zero or one trust mode may
// be configured:
//
//   - WithTrustedHeader trusts one named header verbatim, for platforms
//     whose edge sets a header this process can only receive by going
//     through that edge (e.g. fly.io's Fly-Client-IP -- see
//     NewFlyClientIPResolver).
//   - WithTrustedProxyCIDRs walks X-Forwarded-For, or failing that
//     Forwarded, from the right against a caller-supplied set of trusted
//     proxy ranges, for platforms that publish stable ranges (AWS ALB,
//     Azure Application Gateway, Cloudflare, self-hosted nginx, etc).
//
// If both are configured, WithTrustedHeader takes precedence. If neither is
// configured, ClientAddr is the request's direct peer address (RemoteAddr)
// only -- it never trusts a client-supplied header, unlike GetClientIP.
//
// When the listener understands the PROXY protocol (see WithProxyProtocol),
// RemoteAddr is already the real client, so the zero-configuration resolver
// is the right choice behind such a proxy.
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

// WithTrustedProxyCIDRs configures the resolver to walk X-Forwarded-For (or
// Forwarded, when that is absent) from the right, skipping entries whose
// address falls within the given CIDR ranges, and returning the first entry
// that doesn't -- the RFC 7239-style trusted-proxy-list algorithm. The
// request's direct peer (RemoteAddr) must also fall within the trusted set
// for the headers to be consulted at all; otherwise the peer is untrusted and
// is used directly, since any header it presents is unverified.
//
// An entry that cannot be parsed ends the walk: it was written by a hop this
// process cannot vouch for, so the nearest address before it is returned.
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

// ClientAddr resolves r's client address under the configured trust mode. It
// returns the zero Addr when neither the headers nor RemoteAddr yield a valid
// address. IPv4-mapped IPv6 addresses are returned as IPv4 and zones are
// dropped, so the result is a stable key.
func (c *ClientIPResolver) ClientAddr(r *http.Request) netip.Addr {
	if c.trustedHeader != "" {
		return c.clientAddrFromTrustedHeader(r)
	}
	if len(c.trustedProxyCIDRs) > 0 {
		return c.clientAddrFromTrustedProxyCIDRs(r)
	}

	return remoteAddrIP(r)
}

// ClientIP resolves r's client address as a string, falling back to the raw
// RemoteAddr when it cannot be parsed.
//
// Deprecated: Use ClientAddr, which returns a netip.Addr.
func (c *ClientIPResolver) ClientIP(r *http.Request) string {
	return c.clientAddrString(r)
}

// PrincipalFunc adapts ClientAddr to the PrincipalFunc signature, for use
// with WithRateLimiterPrincipalFunc/WithPrincipalFunc.
func (c *ClientIPResolver) PrincipalFunc(r *http.Request) (string, error) {
	return c.clientAddrString(r), nil
}

func (c *ClientIPResolver) clientAddrString(r *http.Request) string {
	if addr := c.ClientAddr(r); addr.IsValid() {
		return addr.String()
	}

	return r.RemoteAddr
}

func (c *ClientIPResolver) clientAddrFromTrustedHeader(r *http.Request) netip.Addr {
	if addr, ok := parseForwardedIP(r.Header.Get(c.trustedHeader)); ok {
		return addr
	}
	// The trusted header is absent or unusable (e.g. something hit this
	// process directly, bypassing the edge that normally sets it).
	return remoteAddrIP(r)
}

func (c *ClientIPResolver) clientAddrFromTrustedProxyCIDRs(r *http.Request) netip.Addr {
	peer := remoteAddrIP(r)
	if !peer.IsValid() || !c.isTrustedProxy(peer) {
		// The immediate peer isn't a trusted proxy (or couldn't be
		// parsed), so any header it presents is unverified -- the peer
		// itself is the only address that can be trusted.
		return peer
	}

	hops := forwardedHops(r)

	// nearest is the closest hop already known to be a trusted proxy.
	nearest := peer
	for _, hop := range slices.Backward(hops) {
		addr, ok := parseForwardedIP(hop)
		if !ok {
			return nearest
		}
		if !c.isTrustedProxy(addr) {
			return addr
		}
		nearest = addr
	}

	// Every entry (or none at all) was a trusted proxy.
	return peer
}

func (c *ClientIPResolver) isTrustedProxy(addr netip.Addr) bool {
	for _, cidr := range c.trustedProxyCIDRs {
		if cidr.Contains(addr) {
			return true
		}
	}

	return false
}

// forwardedHops returns the hop addresses from X-Forwarded-For, or failing
// that the for= values of Forwarded, in the order the proxies appended them.
func forwardedHops(r *http.Request) []string {
	if xff := r.Header.Values("X-Forwarded-For"); len(xff) > 0 {
		return splitHeaderList(xff)
	}

	var hops []string
	for _, h := range splitHeaderList(r.Header.Values("Forwarded")) {
		hops = append(hops, forwardedFor(h))
	}

	return hops
}

// splitHeaderList splits repeated, comma-separated header values into
// elements, in order.
func splitHeaderList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, strings.Split(v, ",")...)
	}

	return out
}

// forwardedFor returns the for= value of one Forwarded element, or "" when it
// has none.
func forwardedFor(element string) string {
	for part := range strings.SplitSeq(element, ";") {
		before, after, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && strings.EqualFold(before, "for") {
			return strings.Trim(strings.TrimSpace(after), "\"")
		}
	}

	return ""
}

// remoteAddrIP extracts and parses the IP portion of r.RemoteAddr, which
// net/http guarantees to be in "IP:port" form. It returns the zero Addr when
// that cannot be parsed.
func remoteAddrIP(r *http.Request) netip.Addr {
	addr, _ := parseForwardedIP(r.RemoteAddr)

	return addr
}

// parseForwardedIP parses a single address as it appears in a forwarding
// header or RemoteAddr, tolerating surrounding whitespace, an optional port
// and bracketed IPv6 literals. The result is unmapped and has no zone.
func parseForwardedIP(s string) (netip.Addr, bool) {
	s = strings.TrimSpace(s)
	if ap, err := netip.ParseAddrPort(s); err == nil {
		return normaliseAddr(ap.Addr()), true
	}
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, false
	}

	return normaliseAddr(addr), true
}

func normaliseAddr(addr netip.Addr) netip.Addr {
	return addr.Unmap().WithZone("")
}
