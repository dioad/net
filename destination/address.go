// Package destination decides which network destinations a client may connect
// to, for services that connect outbound to addresses an untrusted party can
// influence (webhook delivery, reverse proxies, SOCKS gateways).
//
// The decision is made at the socket, after name resolution and before the
// connection is made, so it covers redirects, DNS rebinding and a hostname
// that resolves to several addresses. A check at configuration time is a
// convenience only.
package destination

import (
	"net/netip"
	"slices"
)

// nonPublicPrefixes are the special-purpose ranges, beyond those netip.Addr
// classifies itself, that are never reachable as public internet hosts.
var nonPublicPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),       // "this network"
	netip.MustParsePrefix("100.64.0.0/10"),   // carrier-grade NAT, also used by overlay networks
	netip.MustParsePrefix("192.0.0.0/24"),    // IETF protocol assignments
	netip.MustParsePrefix("192.0.2.0/24"),    // documentation
	netip.MustParsePrefix("198.18.0.0/15"),   // benchmarking
	netip.MustParsePrefix("198.51.100.0/24"), // documentation
	netip.MustParsePrefix("203.0.113.0/24"),  // documentation
	netip.MustParsePrefix("240.0.0.0/4"),     // reserved, and broadcast
	netip.MustParsePrefix("64:ff9b:1::/48"),  // local-use IPv4/IPv6 translation
	netip.MustParsePrefix("100::/64"),        // discard-only
	netip.MustParsePrefix("2001::/23"),       // IETF protocol assignments, including Teredo
	netip.MustParsePrefix("2001:db8::/32"),   // documentation
}

// cloudMetadataPrefixes are the addresses cloud providers serve instance
// metadata and credentials from.
var cloudMetadataPrefixes = []netip.Prefix{
	netip.MustParsePrefix("169.254.0.0/16"), // link-local, used by AWS, Azure, GCP and others
	netip.MustParsePrefix("fd00:ec2::/32"),  // AWS IPv6 metadata endpoint
}

// nat64Prefix embeds an IPv4 address in its last 32 bits, so an address in it
// is only as public as the IPv4 address it carries.
var nat64Prefix = netip.MustParsePrefix("64:ff9b::/96")

// canonical returns the address a destination is judged by: IPv4-mapped IPv6
// and NAT64 addresses are replaced by the IPv4 address they carry, and a zone
// is dropped.
func canonical(addr netip.Addr) netip.Addr {
	addr = addr.Unmap().WithZone("")
	if nat64Prefix.Contains(addr) {
		b := addr.As16()

		return netip.AddrFrom4([4]byte(b[12:]))
	}

	return addr
}

// IsPublicAddress reports whether addr is a public internet address: not
// loopback, private (including IPv6 unique local addresses, and so Fly.io's
// private network), link-local (including cloud metadata), multicast,
// unspecified or in a special-use range. IPv4-mapped and NAT64 addresses are
// judged by the IPv4 address they carry.
func IsPublicAddress(addr netip.Addr) bool {
	addr = canonical(addr)
	if !addr.IsValid() || !addr.IsGlobalUnicast() || addr.IsPrivate() {
		return false
	}

	return !containsAddr(nonPublicPrefixes, addr)
}

// IsCloudMetadataAddress reports whether addr is where a cloud provider serves
// instance metadata. IPv4-mapped and NAT64 addresses are judged by the IPv4
// address they carry.
func IsCloudMetadataAddress(addr netip.Addr) bool {
	return containsAddr(cloudMetadataPrefixes, canonical(addr))
}

func containsAddr(prefixes []netip.Prefix, addr netip.Addr) bool {
	return slices.ContainsFunc(prefixes, func(p netip.Prefix) bool { return p.Contains(addr) })
}
