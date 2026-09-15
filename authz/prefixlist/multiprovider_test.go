package prefixlist

import (
	"net/netip"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiProvider_ContainsFetchesOnDemand(t *testing.T) {
	provider := &mockProvider{name: "test", prefixes: []string{"10.0.0.0/8"}}
	m := NewMultiProvider([]Provider{provider}, zerolog.Nop())

	addr, err := netip.ParseAddr("10.1.2.3")
	require.NoError(t, err)

	// Contains must be usable immediately after construction, the same way
	// every other Provider in this package works -- without the caller
	// first having to call Prefixes explicitly to populate a cache.
	assert.True(t, m.Contains(addr), "Contains should fetch prefixes on demand like the other providers")
}

func TestMultiProvider_ContainsReflectsLatestPrefixes(t *testing.T) {
	provider := &mockProvider{name: "test", prefixes: []string{"10.0.0.0/8"}}
	m := NewMultiProvider([]Provider{provider}, zerolog.Nop())

	outside, err := netip.ParseAddr("192.168.1.1")
	require.NoError(t, err)
	assert.False(t, m.Contains(outside))

	// Simulate the provider's upstream list changing between calls.
	provider.prefixes = []string{"192.168.0.0/16"}
	assert.True(t, m.Contains(outside))
}
