package destination

import (
	"net/netip"
	"testing"

	"github.com/dioad/config/configcheck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Config_Check(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"empty", Config{}, false},
		{"valid", Config{AllowPrefixes: []string{"10.0.0.0/8", "fd00::/8"}, AllowedPorts: []int{80, 443}}, false},
		{"bad allow prefix", Config{AllowPrefixes: []string{"10.0.0.1"}}, true},
		{"bad only prefix", Config{OnlyPrefixes: []string{"nope"}}, true},
		{"allow with only", Config{AllowPrefixes: []string{"10.0.0.0/8"}, OnlyPrefixes: []string{"10.0.0.0/8"}}, true},
		{"port zero", Config{AllowedPorts: []int{0}}, true},
		{"port too high", Config{AllowedPorts: []int{65536}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := configcheck.NewCollector()
			tt.cfg.Check(c)

			if tt.wantErr {
				require.Error(t, c.Err())
			} else {
				require.NoError(t, c.Err())
			}
		})
	}
}

func Test_Config_Policy(t *testing.T) {
	policy, err := Config{
		AllowPrefixes:      []string{"10.0.0.0/8"},
		AllowedPorts:       []int{443},
		AllowCloudMetadata: true,
	}.Policy()

	require.NoError(t, err)
	assert.Equal(t, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}, policy.AllowPrefixes)
	assert.Empty(t, policy.OnlyPrefixes)
	assert.Equal(t, []int{443}, policy.AllowedPorts)
	assert.True(t, policy.AllowCloudMetadata)

	_, err = Config{OnlyPrefixes: []string{"nope"}}.Policy()
	require.Error(t, err)
}
