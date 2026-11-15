package destination

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_IsPublicAddress(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want bool
	}{
		{"public IPv4", "8.8.8.8", true},
		{"public IPv4 next to private", "172.32.0.1", true},
		{"public IPv6", "2606:4700:4700::1111", true},
		{"loopback IPv4", "127.0.0.1", false},
		{"loopback IPv6", "::1", false},
		{"RFC 1918 10/8", "10.1.2.3", false},
		{"RFC 1918 172.16/12", "172.16.0.1", false},
		{"RFC 1918 192.168/16", "192.168.1.1", false},
		{"CGNAT", "100.64.0.1", false},
		{"link-local IPv4", "169.254.1.1", false},
		{"cloud metadata", "169.254.169.254", false},
		{"IPv6 ULA", "fd12:3456:789a::1", false},
		{"IPv6 link-local", "fe80::1", false},
		{"IPv4 multicast", "224.0.0.1", false},
		{"IPv6 multicast", "ff02::1", false},
		{"IPv4 unspecified", "0.0.0.0", false},
		{"IPv6 unspecified", "::", false},
		{"this network", "0.1.2.3", false},
		{"IETF protocol assignments", "192.0.0.8", false},
		{"documentation TEST-NET-1", "192.0.2.1", false},
		{"benchmarking", "198.18.0.1", false},
		{"documentation TEST-NET-2", "198.51.100.1", false},
		{"documentation TEST-NET-3", "203.0.113.1", false},
		{"reserved", "240.0.0.1", false},
		{"broadcast", "255.255.255.255", false},
		{"local-use translation", "64:ff9b:1::1", false},
		{"discard-only", "100::1", false},
		{"Teredo", "2001:0:4136:e378:8000:63bf:3fff:fdd2", false},
		{"IPv6 documentation", "2001:db8::1", false},
		{"IPv4-mapped public", "::ffff:8.8.8.8", true},
		{"IPv4-mapped loopback", "::ffff:127.0.0.1", false},
		{"IPv4-mapped private", "::ffff:10.0.0.1", false},
		{"NAT64 public", "64:ff9b::808:808", true},
		{"NAT64 private", "64:ff9b::a00:1", false},
		{"NAT64 metadata", "64:ff9b::a9fe:a9fe", false},
		{"zoned link-local", "fe80::1%eth0", false},
		{"zero value", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var addr netip.Addr
			if tt.addr != "" {
				addr = netip.MustParseAddr(tt.addr)
			}

			assert.Equal(t, tt.want, IsPublicAddress(addr))
		})
	}
}

func Test_IsCloudMetadataAddress(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{"169.254.169.254", true},
		{"169.254.0.1", true},
		{"::ffff:169.254.169.254", true},
		{"64:ff9b::a9fe:a9fe", true},
		{"fd00:ec2::254", true},
		{"10.0.0.1", false},
		{"8.8.8.8", false},
	}
	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			assert.Equal(t, tt.want, IsCloudMetadataAddress(netip.MustParseAddr(tt.addr)))
		})
	}
}
