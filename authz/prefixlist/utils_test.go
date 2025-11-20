package prefixlist

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCIDRs(t *testing.T) {
	tests := []struct {
		name    string
		cidrs   []string
		wantLen int
		wantErr bool
	}{
		{
			name:    "valid IPv4 CIDRs",
			cidrs:   []string{"192.168.1.0/24", "10.0.0.0/8"},
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "valid IPv6 CIDRs",
			cidrs:   []string{"2001:db8::/32", "fe80::/10"},
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "mixed IPv4 and IPv6",
			cidrs:   []string{"192.168.1.0/24", "2001:db8::/32"},
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "with duplicates",
			cidrs:   []string{"192.168.1.0/24", "192.168.1.0/24", "10.0.0.0/8"},
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "invalid CIDR",
			cidrs:   []string{"not-a-cidr"},
			wantErr: true,
		},
		{
			name:    "empty list",
			cidrs:   []string{},
			wantLen: 0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCIDRs(tt.cidrs)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, got, tt.wantLen)
		})
	}
}
