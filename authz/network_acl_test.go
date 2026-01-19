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
	_, cidrTwo, err := net.ParseCIDR("10.0.0.0/30")

	if err != nil {
		t.Fatalf("failed to parse cidr")
	}
	list := []*net.IPNet{
		cidrOne,
		cidrTwo,
	}

	addrOne := net.ParseIP("127.0.0.123")

	gotOne := containsAddress(list, addrOne)
	require.Equal(t, gotOne, true)

	addrTwo := net.ParseIP("10.0.0.1")

	gotTwo := containsAddress(list, addrTwo)
	require.Equal(t, gotTwo, true)

	addrThree := net.ParseIP("192.164.12.45")

	gotThree := containsAddress(list, addrThree)
	require.Equal(t, gotThree, false)
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

	got, err := a.AuthoriseFromString("192.168.4.5:12345")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	require.False(t, got)
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

	got, err := a.AuthoriseFromString("192.168.4.5:12354")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	require.True(t, got)
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

	got, err := a.AuthoriseFromString("192.168.4.5:1234")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	require.True(t, got)
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
	got, err := a.AuthoriseFromString("[2001:db8::1]:1234")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	require.True(t, got)
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
	got, err := a.AuthoriseFromString("[2001:db9::1]:1234")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	require.False(t, got)
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
	got, err := a.AuthoriseFromString("[2001:db8::1]:1234")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	require.True(t, got)

	// Test with a different IPv6 address
	got, err = a.AuthoriseFromString("[2001:db8::2]:1234")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	require.False(t, got)
}
