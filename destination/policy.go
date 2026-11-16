package destination

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"syscall"
)

// ErrDenied is returned, wrapped, when a policy does not allow a destination.
// Callers can map it to a validation error or a refused request with
// errors.Is.
var ErrDenied = errors.New("destination not allowed")

// Policy decides which destinations a client may connect to. The zero value
// allows only public addresses on any port.
//
// Cloud metadata endpoints are denied unless AllowCloudMetadata is set,
// whatever the prefixes say, so a broad allow-list cannot expose them by
// accident.
//
// An address is judged in this order: cloud metadata (unless
// AllowCloudMetadata), then DenyPrefixes, then OnlyPrefixes, or AllowPrefixes
// and the public-address default. A deny range is final: no allow range
// overrides it.
type Policy struct {
	// AllowPrefixes are exceptions to the default deny of non-public
	// addresses, for example the private network of a deployment.
	AllowPrefixes []netip.Prefix

	// OnlyPrefixes, when set, allows only these ranges and replaces the
	// public-address default. AllowPrefixes has no effect with it.
	OnlyPrefixes []netip.Prefix

	// DenyPrefixes are refused even when OnlyPrefixes or AllowPrefixes contains
	// them, for an address inside an allowed range that must stay unreachable,
	// such as a platform service on a private network.
	DenyPrefixes []netip.Prefix

	// AllowedPorts, when set, restricts destination ports.
	AllowedPorts []int

	// AllowCloudMetadata permits cloud metadata endpoints, which are denied
	// otherwise.
	AllowCloudMetadata bool
}

// Check returns an error wrapping ErrDenied when the policy does not allow
// connecting to dest. IPv4-mapped and NAT64 addresses are judged by the IPv4
// address they carry.
func (p Policy) Check(dest netip.AddrPort) error {
	if len(p.AllowedPorts) > 0 && !slices.Contains(p.AllowedPorts, int(dest.Port())) {
		return fmt.Errorf("%w: port %d is not an allowed port", ErrDenied, dest.Port())
	}

	addr := canonical(dest.Addr())
	if !addr.IsValid() {
		return fmt.Errorf("%w: %q is not an address", ErrDenied, dest.Addr())
	}

	return p.checkAddr(addr)
}

// checkAddr applies the address rules to a canonical, valid addr.
func (p Policy) checkAddr(addr netip.Addr) error {
	metadata := IsCloudMetadataAddress(addr)
	if metadata && !p.AllowCloudMetadata {
		return fmt.Errorf("%w: %s is a cloud metadata endpoint", ErrDenied, addr)
	}

	if containsAddr(p.DenyPrefixes, addr) {
		return fmt.Errorf("%w: %s is in a denied range", ErrDenied, addr)
	}

	if len(p.OnlyPrefixes) > 0 {
		if !containsAddr(p.OnlyPrefixes, addr) {
			return fmt.Errorf("%w: %s is not in an allowed range", ErrDenied, addr)
		}

		return nil
	}
	if metadata || containsAddr(p.AllowPrefixes, addr) || IsPublicAddress(addr) {
		return nil
	}

	return fmt.Errorf("%w: %s is not a public address", ErrDenied, addr)
}

// Control is a net.Dialer.Control hook. It checks the numeric address a dial
// is about to connect to, after name resolution and before the socket
// connects, so no path to a denied address remains however the dial was
// reached.
func (p Policy) Control(_, address string, _ syscall.RawConn) error {
	dest, err := netip.ParseAddrPort(address)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDenied, err)
	}

	return p.Check(dest)
}

// Dialer returns a net.Dialer that enforces the policy on every address it
// connects to.
func (p Policy) Dialer() *net.Dialer {
	return &net.Dialer{Control: p.Control}
}
