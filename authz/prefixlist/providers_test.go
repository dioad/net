package prefixlist

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitLabProvider(t *testing.T) {
	provider := NewGitLabProvider()

	assert.Equal(t, "gitlab", provider.Name())

	ctx := context.Background()
	prefixes, err := provider.Prefixes(ctx)
	require.NoError(t, err)
	assert.Len(t, prefixes, 2)

	// Verify the static IPs are parsed correctly
	expectedCIDRs := []string{"34.74.90.64/28", "34.74.226.0/24"}
	for i, expected := range expectedCIDRs {
		assert.Equal(t, expected, prefixes[i].String())
	}
}

func TestHetznerProvider(t *testing.T) {
	provider := NewHetznerProvider()

	assert.Equal(t, "hetzner", provider.Name())

	ctx := context.Background()
	prefixes, err := provider.Prefixes(ctx)
	require.NoError(t, err)
	assert.Greater(t, len(prefixes), 30, "Expected at least 30 Hetzner prefixes")

	// Verify at least one known Hetzner range is included
	found := false
	for _, prefix := range prefixes {
		if prefix.String() == "5.9.0.0/16" {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected to find Hetzner range 5.9.0.0/16")
}

func TestProviderNames(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		expected string
	}{
		{
			name:     "github no filter",
			provider: NewGitHubProvider(""),
			expected: "github",
		},
		{
			name:     "github with filter",
			provider: NewGitHubProvider("hooks"),
			expected: "github-hooks",
		},
		{
			name:     "cloudflare ipv4",
			provider: NewCloudflareProvider(false),
			expected: "cloudflare-ipv4",
		},
		{
			name:     "cloudflare ipv6",
			provider: NewCloudflareProvider(true),
			expected: "cloudflare-ipv6",
		},
		{
			name:     "google",
			provider: NewGoogleProvider(nil, nil),
			expected: "google",
		},
		{
			name:     "atlassian",
			provider: NewAtlassianProvider(nil, nil),
			expected: "atlassian",
		},
		{
			name:     "aws no filter",
			provider: NewAWSProvider("", ""),
			expected: "aws",
		},
		{
			name:     "aws with service",
			provider: NewAWSProvider("EC2", ""),
			expected: "aws-EC2",
		},
		{
			name:     "aws with service and region",
			provider: NewAWSProvider("EC2", "us-east-1"),
			expected: "aws-EC2-us-east-1",
		},
		{
			name:     "fastly",
			provider: NewFastlyProvider(),
			expected: "fastly",
		},
		{
			name:     "hetzner",
			provider: NewHetznerProvider(),
			expected: "hetzner",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.provider.Name())
		})
	}
}
