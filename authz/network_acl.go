// Package authz provides network-based and principal-based access control utilities.
package authz

import (
	"fmt"
	"net"
	"strings"

	"github.com/dioad/generics"
)

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
func (a *NetworkACL) Deny(net *net.IPNet) {
	a.denyNetworks = append(a.denyNetworks, net)
}

// AuthoriseConn checks if the provided connection is authorised.
func (a *NetworkACL) AuthoriseConn(c net.Conn) (bool, error) {
	return a.AuthoriseFromString(c.RemoteAddr().String())
}

// AuthoriseFromString checks if the provided address string is authorised.
func (a *NetworkACL) AuthoriseFromString(addr string) (bool, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return false, err
	}
	return a.Authorise(tcpAddr), nil
}

// Authorise checks if the provided TCP address is authorised.
// If both allow and deny lists are present, allow is checked first.
// If an IP is in the allow list but also matches a deny rule, authorisation is denied.
// This allows denying subsets of allowed CIDR ranges.
func (a *NetworkACL) Authorise(addr *net.TCPAddr) bool {
	inAllow := containsAddress(a.allowNetworks, addr.IP)
	inDeny := containsAddress(a.denyNetworks, addr.IP)

	if inAllow && !inDeny {
		return true
	}

	// if in both allow and deny, deny
	if inAllow {
		return false
	}

	if inDeny {
		return false
	}

	return a.AllowByDefault
}

func containsAddress(netList []*net.IPNet, ip net.IP) bool {
	for _, n := range netList {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func parseTCPNet(n string) (*net.IPNet, error) {
	netParts := strings.Split(n, "/")
	if len(netParts) == 1 {
		n = fmt.Sprintf("%v/32", n)
	}

	_, ipNet, err := net.ParseCIDR(n)
	if err != nil {
		return nil, err
	}

	return ipNet, nil
}
