package http

import (
	"net/netip"

	"github.com/dioad/config/configcheck"
)

// ClientIPConfig is the trust model of ClientIPResolver, in the form services
// take it in their configuration. An empty config trusts no header, so the
// client address is the connection's address.
type ClientIPConfig struct {
	// TrustedProxies are the ranges of the proxies in front of the service,
	// in CIDR notation. Forwarding headers are honoured only from a peer in
	// one of them.
	TrustedProxies []string `desc:"CIDR ranges of proxies whose forwarding headers are trusted" mapstructure:"trusted-proxies"`

	// TrustedHeader names a header set by an edge that this process can only
	// be reached through, such as Fly-Client-IP on fly.io. It takes
	// precedence over TrustedProxies.
	TrustedHeader string `desc:"header set by the edge proxy that is trusted verbatim" mapstructure:"trusted-header"`
}

// Check reports to c a trusted proxy that is not a CIDR range, and trust
// modes set together. c is positioned at the config's own block.
func (c ClientIPConfig) Check(findings *configcheck.Collector) {
	for i, p := range c.TrustedProxies {
		if _, err := netip.ParsePrefix(p); err != nil {
			findings.Field("trusted-proxies").Index(i).Errorf("%q is not a CIDR range: %v", p, err)
		}
	}
	if c.TrustedHeader != "" && len(c.TrustedProxies) > 0 {
		findings.Field("trusted-header").Errorf("is set together with trusted-proxies, which it takes precedence over; set only one")
	}
}

// Resolver returns the ClientIPResolver for c. It returns an error for a
// trusted proxy that is not a CIDR range; Check reports the same.
func (c ClientIPConfig) Resolver() (*ClientIPResolver, error) {
	var opts []ClientIPResolverOption
	if c.TrustedHeader != "" {
		opts = append(opts, WithTrustedHeader(c.TrustedHeader))
	}

	prefixes := make([]netip.Prefix, 0, len(c.TrustedProxies))
	for _, p := range c.TrustedProxies {
		prefix, err := netip.ParsePrefix(p)
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, prefix)
	}
	if len(prefixes) > 0 {
		opts = append(opts, WithTrustedProxyCIDRs(prefixes...))
	}

	return NewClientIPResolver(opts...), nil
}
