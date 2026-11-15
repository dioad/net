package destination

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prefixes(values ...string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(values))
	for _, v := range values {
		out = append(out, netip.MustParsePrefix(v))
	}

	return out
}

func Test_Policy_Check(t *testing.T) {
	tests := []struct {
		name   string
		policy Policy
		dest   string
		denied bool
	}{
		{"zero value allows public", Policy{}, "8.8.8.8:443", false},
		{"zero value denies loopback", Policy{}, "127.0.0.1:80", true},
		{"zero value denies private", Policy{}, "10.0.0.1:80", true},
		{"zero value denies metadata", Policy{}, "169.254.169.254:80", true},
		{"zero value denies mapped loopback", Policy{}, "[::ffff:127.0.0.1]:80", true},
		{"allow prefix permits private", Policy{AllowPrefixes: prefixes("10.0.0.0/8")}, "10.1.1.1:80", false},
		{"allow prefix does not permit other private", Policy{AllowPrefixes: prefixes("10.0.0.0/8")}, "192.168.1.1:80", true},
		{"allow prefix permits mapped form", Policy{AllowPrefixes: prefixes("10.0.0.0/8")}, "[::ffff:10.1.1.1]:80", false},
		{"allow prefix permits NAT64 form", Policy{AllowPrefixes: prefixes("10.0.0.0/8")}, "[64:ff9b::a01:101]:80", false},
		{"allow prefix still permits public", Policy{AllowPrefixes: prefixes("10.0.0.0/8")}, "8.8.8.8:80", false},
		{"broad allow prefix does not expose metadata", Policy{AllowPrefixes: prefixes("169.254.0.0/16")}, "169.254.169.254:80", true},
		{"allow prefix with flag exposes metadata", Policy{AllowPrefixes: prefixes("169.254.0.0/16"), AllowCloudMetadata: true}, "169.254.169.254:80", false},
		{"flag alone exposes metadata", Policy{AllowCloudMetadata: true}, "169.254.169.254:80", false},
		{"flag does not expose other link-local", Policy{AllowCloudMetadata: true}, "[fe80::1]:80", true},
		{"only denies public", Policy{OnlyPrefixes: prefixes("10.0.0.0/8")}, "8.8.8.8:80", true},
		{"only permits listed", Policy{OnlyPrefixes: prefixes("10.0.0.0/8")}, "10.0.0.1:80", false},
		{"only ignores allow", Policy{OnlyPrefixes: prefixes("10.0.0.0/8"), AllowPrefixes: prefixes("192.168.0.0/16")}, "192.168.0.1:80", true},
		{"only denies metadata", Policy{OnlyPrefixes: prefixes("169.254.0.0/16")}, "169.254.169.254:80", true},
		{"only with flag permits metadata", Policy{OnlyPrefixes: prefixes("169.254.0.0/16"), AllowCloudMetadata: true}, "169.254.169.254:80", false},
		{"allowed port", Policy{AllowedPorts: []int{443}}, "8.8.8.8:443", false},
		{"other port", Policy{AllowedPorts: []int{443}}, "8.8.8.8:80", true},
		{"port checked with only", Policy{OnlyPrefixes: prefixes("10.0.0.0/8"), AllowedPorts: []int{443}}, "10.0.0.1:80", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Check(netip.MustParseAddrPort(tt.dest))

			if tt.denied {
				require.ErrorIs(t, err, ErrDenied)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_Policy_Check_ZeroAddr(t *testing.T) {
	require.ErrorIs(t, Policy{}.Check(netip.AddrPort{}), ErrDenied)
}

func Test_Policy_Control(t *testing.T) {
	t.Run("denied address", func(t *testing.T) {
		require.ErrorIs(t, Policy{}.Control("tcp4", "127.0.0.1:80", nil), ErrDenied)
	})
	t.Run("allowed address", func(t *testing.T) {
		require.NoError(t, Policy{}.Control("tcp4", "8.8.8.8:80", nil))
	})
	t.Run("zoned address", func(t *testing.T) {
		require.ErrorIs(t, Policy{}.Control("tcp6", "[fe80::1%en0]:80", nil), ErrDenied)
	})
	t.Run("not a numeric address", func(t *testing.T) {
		err := Policy{}.Control("tcp", "example.com:80", nil)

		require.ErrorIs(t, err, ErrDenied)
		assert.Error(t, err)
	})
}
