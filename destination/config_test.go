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
		{"valid deny", Config{OnlyPrefixes: []string{"fdaa::/16"}, DenyPrefixes: []string{"fdaa::3/128"}}, false},
		{"bad deny prefix", Config{DenyPrefixes: []string{"fdaa::3"}}, true},
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
		DenyPrefixes:       []string{"10.1.0.0/16"},
		AllowedPorts:       []int{443},
		AllowCloudMetadata: true,
	}.Policy()

	require.NoError(t, err)
	assert.Equal(t, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}, policy.AllowPrefixes)
	assert.Empty(t, policy.OnlyPrefixes)
	assert.Equal(t, []netip.Prefix{netip.MustParsePrefix("10.1.0.0/16")}, policy.DenyPrefixes)
	assert.Equal(t, []int{443}, policy.AllowedPorts)
	assert.True(t, policy.AllowCloudMetadata)

	_, err = Config{OnlyPrefixes: []string{"nope"}}.Policy()
	require.Error(t, err)
}

func Test_Config_Check_DenyPath(t *testing.T) {
	c := configcheck.NewCollector()
	Config{DenyPrefixes: []string{"ok", "10.0.0.0/8", "bad"}}.Check(c)

	findings := c.Findings()
	require.Len(t, findings, 2)
	assert.Equal(t, "deny[0]", findings[0].Path)
	assert.Equal(t, "deny[2]", findings[1].Path)
}

func Test_Config_Policy_DenyError(t *testing.T) {
	_, err := Config{DenyPrefixes: []string{"nope"}}.Policy()
	require.Error(t, err)
}
