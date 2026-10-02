// Package authz provides network-based and principal-based access control utilities.
package authz

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/dioad/generics"
)

// ErrDenied is the sentinel a *DeniedError always unwraps to via errors.Is,
// so callers can distinguish a deliberate ACL denial from an operational
// failure (e.g. an unparsable address) without inspecting error text.
var ErrDenied = errors.New("denied by network ACL")

// DeniedError describes why a NetworkACL denied an address. Retrieve it with
// errors.As to log the specific network/default that decided the outcome.
type DeniedError struct {
	// MatchedNetwork is the specific CIDR that caused the denial - an
	// explicit deny-list entry, including one that overrode an allow-list
	// match (deny wins). Empty when the address matched neither list and
	// the result came from falling through to AllowByDefault=false.
	MatchedNetwork string

	// AllowByDefault is the ACL's configured fallback, included so a log
	// line built from this error doesn't need the *NetworkACL in scope to
	// explain a no-match denial.
	AllowByDefault bool
}

func (e *DeniedError) Error() string {
	if e.MatchedNetwork != "" {
		return "denied by network ACL: address matched deny network " + e.MatchedNetwork
	}
	return "denied by network ACL: no allow network matched and allow-by-default is false"
}

// Is reports whether target is ErrDenied, so errors.Is(err, ErrDenied) works
// without callers needing to know about the concrete *DeniedError type.
func (e *DeniedError) Is(target error) bool {
	return target == ErrDenied
}

// NetworkACL describes network-based access control rules.
type NetworkACL struct {
	AllowByDefault bool

	allowNetworks []*net.IPNet
	denyNetworks  []*net.IPNet
}

// NewNetworkACL creates a new NetworkACL from the provided configuration.
func NewNetworkACL(cfg NetworkACLConfig) (*NetworkACL, error) {
	allowNetworks, err := generics.Map(parseTCPNet, cfg.AllowedNets)
	if err != nil {
		return nil, fmt.Errorf("failed to parse allowed networks: %w", err)
	}

	denyNetworks, err := generics.Map(parseTCPNet, cfg.DeniedNets)
	if err != nil {
		return nil, fmt.Errorf("failed to parse denied networks: %w", err)
	}

	a := &NetworkACL{
		AllowByDefault: cfg.AllowByDefault,
		allowNetworks:  allowNetworks,
		denyNetworks:   denyNetworks,
	}

	return a, err
}

// AllowFromString parses a network string and adds it to the allow list.
func (a *NetworkACL) AllowFromString(n string) error {
	tcpNet, err := parseTCPNet(n)
	if err != nil {
		return err
	}
	a.Allow(tcpNet)

	return nil
}

// Allow adds a network to the allow list.
func (a *NetworkACL) Allow(n *net.IPNet) {
	a.allowNetworks = append(a.allowNetworks, n)
}

// DenyFromString parses a network string and adds it to the deny list.
func (a *NetworkACL) DenyFromString(n string) error {
	tcpNet, err := parseTCPNet(n)
	if err != nil {
		return err
	}
	a.Deny(tcpNet)

	return nil
}

// Deny adds a network to the deny list.
func (a *NetworkACL) Deny(n *net.IPNet) {
	a.denyNetworks = append(a.denyNetworks, n)
}

// AuthoriseConn checks if the provided connection is authorised. It returns
// nil when authorised, or an error describing why not - see Authorise.
func (a *NetworkACL) AuthoriseConn(c net.Conn) error {
	return a.AuthoriseFromString(c.RemoteAddr().String())
}

// AuthoriseFromString checks if the provided address string is authorised.
// It returns nil when authorised, or an error describing why not - see
// Authorise. A malformed addr is returned unwrapped, so errors.Is(err,
// ErrDenied) is false for it, distinguishing "couldn't evaluate" from "ACL
// said no".
func (a *NetworkACL) AuthoriseFromString(addr string) error {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return err
	}

	return a.Authorise(tcpAddr)
}

// Authorise checks if the provided TCP address is authorised. It returns nil
// when addr is allowed, or a *DeniedError (retrievable with errors.As, and
// matching errors.Is(err, ErrDenied)) describing which rule, or default,
// denied it.
//
// If both allow and deny lists are present, allow is checked first.
// If an IP is in the allow list but also matches a deny rule, authorisation is denied.
// This allows denying subsets of allowed CIDR ranges.
func (a *NetworkACL) Authorise(addr *net.TCPAddr) error {
	// Deny takes precedence over allow, so it's checked first regardless of
	// whether addr also matches an allow network.
	if denyMatch, ok := matchingNetwork(a.denyNetworks, addr.IP); ok {
		return &DeniedError{MatchedNetwork: denyMatch.String()}
	}

	if _, ok := matchingNetwork(a.allowNetworks, addr.IP); ok {
		return nil
	}

	if a.AllowByDefault {
		return nil
	}

	return &DeniedError{AllowByDefault: a.AllowByDefault}
}

// matchingNetwork returns the first network in netList containing ip, so
// callers can report which specific CIDR decided an Authorise outcome.
func matchingNetwork(netList []*net.IPNet, ip net.IP) (*net.IPNet, bool) {
	for _, n := range netList {
		if n.Contains(ip) {
			return n, true
		}
	}

	return nil, false
}

func parseTCPNet(n string) (*net.IPNet, error) {
	netParts := strings.Split(n, "/")
	if len(netParts) == 1 {
		// No mask provided, detect IP version and use appropriate default
		ip := net.ParseIP(n)
		if ip == nil {
			return nil, fmt.Errorf("invalid IP address: %s", n)
		}

		// Check if it's IPv6 (fails to convert to IPv4)
		if ip.To4() == nil {
			// IPv6 address
			n = fmt.Sprintf("%v/128", n)
		} else {
			// IPv4 address
			n = fmt.Sprintf("%v/32", n)
		}
	}

	_, ipNet, err := net.ParseCIDR(n)
	if err != nil {
		return nil, err
	}

	return ipNet, nil
}
