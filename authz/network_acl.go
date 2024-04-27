package authz

import (
	"fmt"
	"net"
	"strings"

	"github.com/dioad/generics"
)

type NetworkACL struct {
	AllowByDefault bool

	allowNetworks []*net.IPNet
	denyNetworks  []*net.IPNet
}

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

func (a *NetworkACL) AllowFromString(n string) error {
	tcpNet, err := parseTCPNet(n)
	if err != nil {
		return err
	}
	a.Allow(tcpNet)
	return nil
}

func (a *NetworkACL) Allow(n *net.IPNet) {
	a.allowNetworks = append(a.allowNetworks, n)
}

func (a *NetworkACL) DenyFromString(n string) error {
	tcpNet, err := parseTCPNet(n)
	if err != nil {
		return err
	}
	a.Deny(tcpNet)
	return nil
}

func (a *NetworkACL) Deny(net *net.IPNet) {
	a.denyNetworks = append(a.denyNetworks, net)
}

func (a *NetworkACL) AuthoriseConn(c net.Conn) (bool, error) {
	return a.AuthoriseFromString(c.RemoteAddr().String())
}

func (a *NetworkACL) AuthoriseFromString(addr string) (bool, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return false, err
	}
	return a.Authorise(tcpAddr), nil
}

// Authorise
// if both
// allow is checked first, if empty
// if ip is in allow but also matches deny, authorisation is denied
// this is to allow people to deny subsets of allowed CIDR ranges.
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

// TODO: investigate if there is a more optimal way
// of searching address space
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
