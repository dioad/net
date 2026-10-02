package authz

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseNetWithDefault(t *testing.T) {
	testCase := "127.0.0.1"

	expectedOnes, expectedBits := net.CIDRMask(32, 32).Size()

	n, err := parseTCPNet(testCase)
	if err != nil {
		t.Fatalf("didn't expect err: %v", err)
	}

	gotOnes, gotBits := n.Mask.Size()

	if gotOnes != expectedOnes {
		t.Fatalf("got %v ones, expected %v ones", gotOnes, expectedOnes)
	}

	if gotBits != expectedBits {
		t.Fatalf("got %v bits, expected %v bits", gotBits, expectedBits)
	}
}

func TestContains(t *testing.T) {
	_, cidrOne, err := net.ParseCIDR("127.0.0.0/24")
	if err != nil {
		t.Fatalf("failed to parse cidr")
	}

	_, cidrTwo, err := net.ParseCIDR("10.0.0.0/30")
	if err != nil {
		t.Fatalf("failed to parse cidr")
	}
	list := []*net.IPNet{
		cidrOne,
		cidrTwo,
	}

	addrOne := net.ParseIP("127.0.0.123")

	matchedOne, gotOne := matchingNetwork(list, addrOne)
	require.True(t, gotOne)
	require.Equal(t, cidrOne, matchedOne)

	addrTwo := net.ParseIP("10.0.0.1")

	matchedTwo, gotTwo := matchingNetwork(list, addrTwo)
	require.True(t, gotTwo)
	require.Equal(t, cidrTwo, matchedTwo)

	addrThree := net.ParseIP("192.164.12.45")

	_, gotThree := matchingNetwork(list, addrThree)
	require.False(t, gotThree)
}

func TestAuthoriserDenyByDefault(t *testing.T) {
	c := NetworkACLConfig{
		AllowedNets:    []string{},
		DeniedNets:     []string{},
		AllowByDefault: false,
	}
	a, err := NewNetworkACL(c)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = a.AuthoriseFromString("192.168.4.5:12345")
	require.ErrorIs(t, err, ErrDenied)
}

func TestAuthoriserAllowByDefault(t *testing.T) {
	c := NetworkACLConfig{
		AllowedNets:    []string{},
		DeniedNets:     []string{},
		AllowByDefault: true,
	}

	a, err := NewNetworkACL(c)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = a.AuthoriseFromString("192.168.4.5:12354")
	require.NoError(t, err)
}

func TestAuthoriserDeniedError_MatchedNetwork(t *testing.T) {
	c := NetworkACLConfig{
		AllowedNets:    []string{"192.168.0.0/16"},
		DeniedNets:     []string{"192.168.4.0/24"},
		AllowByDefault: false,
	}
	a, err := NewNetworkACL(c)
	require.NoError(t, err)

	err = a.AuthoriseFromString("192.168.4.5:1234")
	require.ErrorIs(t, err, ErrDenied)

	var denied *DeniedError
	require.ErrorAs(t, err, &denied)
	require.Equal(t, "192.168.4.0/24", denied.MatchedNetwork)
}

func TestAuthoriserDeniedError_NoMatchUsesDefault(t *testing.T) {
	c := NetworkACLConfig{AllowByDefault: false}
	a, err := NewNetworkACL(c)
	require.NoError(t, err)

	err = a.AuthoriseFromString("203.0.113.1:1234")
	require.ErrorIs(t, err, ErrDenied)

	var denied *DeniedError
	require.ErrorAs(t, err, &denied)
	require.Empty(t, denied.MatchedNetwork)
	require.False(t, denied.AllowByDefault)
}

func TestAuthoriserAllowFromString(t *testing.T) {
	c := NetworkACLConfig{
		AllowedNets:    []string{"192.168.0.0/16"},
		DeniedNets:     []string{},
		AllowByDefault: false,
	}

	a, err := NewNetworkACL(c)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = a.AuthoriseFromString("192.168.4.5:1234")
	require.NoError(t, err)
}

func TestParseNetIPv6WithDefault(t *testing.T) {
	testCase := "2001:db8::1"

	expectedOnes, expectedBits := net.CIDRMask(128, 128).Size()

	n, err := parseTCPNet(testCase)
	require.NoError(t, err)

	gotOnes, gotBits := n.Mask.Size()

	require.Equal(t, expectedOnes, gotOnes)
	require.Equal(t, expectedBits, gotBits)
}

func TestParseNetIPv6WithMask(t *testing.T) {
	testCase := "2001:db8::/32"

	expectedOnes, expectedBits := net.CIDRMask(32, 128).Size()

	n, err := parseTCPNet(testCase)
	require.NoError(t, err)

	gotOnes, gotBits := n.Mask.Size()

	require.Equal(t, expectedOnes, gotOnes)
	require.Equal(t, expectedBits, gotBits)
}

func TestAuthoriserIPv6Allow(t *testing.T) {
	c := NetworkACLConfig{
		AllowedNets:    []string{"2001:db8::/32"},
		DeniedNets:     []string{},
		AllowByDefault: false,
	}

	a, err := NewNetworkACL(c)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Test with IPv6 address in the allowed range
	err = a.AuthoriseFromString("[2001:db8::1]:1234")
	require.NoError(t, err)
}

func TestAuthoriserIPv6Deny(t *testing.T) {
	c := NetworkACLConfig{
		AllowedNets:    []string{"2001:db8::/32"},
		DeniedNets:     []string{},
		AllowByDefault: false,
	}

	a, err := NewNetworkACL(c)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Test with IPv6 address outside the allowed range
	err = a.AuthoriseFromString("[2001:db9::1]:1234")
	require.ErrorIs(t, err, ErrDenied)
}

func TestAuthoriserIPv6SingleAddress(t *testing.T) {
	c := NetworkACLConfig{
		AllowedNets:    []string{"2001:db8::1"},
		DeniedNets:     []string{},
		AllowByDefault: false,
	}

	a, err := NewNetworkACL(c)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Test with the exact IPv6 address
	err = a.AuthoriseFromString("[2001:db8::1]:1234")
	require.NoError(t, err)

	// Test with a different IPv6 address
	err = a.AuthoriseFromString("[2001:db8::2]:1234")
	require.ErrorIs(t, err, ErrDenied)
}
