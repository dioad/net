package prefixlist

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitLabProvider(t *testing.T) {
	provider := NewGitLabProvider()

	assert.Equal(t, "gitlab", provider.Name())
	assert.Equal(t, 7*24*time.Hour, provider.CacheDuration())

	ctx := context.Background()
	prefixes, err := provider.FetchPrefixes(ctx)
	require.NoError(t, err)
	assert.Len(t, prefixes, 2)

	// Verify the static IPs are parsed correctly
	expectedCIDRs := []string{"34.74.90.64/28", "34.74.226.0/24"}
	for i, expected := range expectedCIDRs {
		_, expectedNet, _ := net.ParseCIDR(expected)
		assert.Equal(t, expectedNet.String(), prefixes[i].String())
	}
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
			provider: NewGoogleProvider(),
			expected: "google",
		},
		{
			name:     "atlassian",
			provider: NewAtlassianProvider(),
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.provider.Name())
		})
	}
}

func TestCacheDurations(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		expected time.Duration
	}{
		{
			name:     "github",
			provider: NewGitHubProvider(""),
			expected: 1 * time.Hour,
		},
		{
			name:     "cloudflare",
			provider: NewCloudflareProvider(false),
			expected: 24 * time.Hour,
		},
		{
			name:     "google",
			provider: NewGoogleProvider(),
			expected: 24 * time.Hour,
		},
		{
			name:     "atlassian",
			provider: NewAtlassianProvider(),
			expected: 24 * time.Hour,
		},
		{
			name:     "gitlab",
			provider: NewGitLabProvider(),
			expected: 7 * 24 * time.Hour,
		},
		{
			name:     "aws",
			provider: NewAWSProvider("", ""),
			expected: 24 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.provider.CacheDuration())
		})
	}
}

func TestParseAWSFilter(t *testing.T) {
	tests := []struct {
		name           string
		filter         string
		wantService    string
		wantRegion     string
	}{
		{
			name:        "empty filter",
			filter:      "",
			wantService: "",
			wantRegion:  "",
		},
		{
			name:        "service only",
			filter:      "EC2",
			wantService: "EC2",
			wantRegion:  "",
		},
		{
			name:        "service and region",
			filter:      "EC2:us-east-1",
			wantService: "EC2",
			wantRegion:  "us-east-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, region := parseAWSFilter(tt.filter)
			assert.Equal(t, tt.wantService, service)
			assert.Equal(t, tt.wantRegion, region)
		})
	}
}
