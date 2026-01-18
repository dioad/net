package prefixlist

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegrationProviders tests actual provider endpoints (skipped by default)
func TestIntegrationProviders(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tests := []struct {
		name     string
		provider Provider
		// expectedMinPrefixes is the minimum number of prefixes we expect
		// This helps catch if the API response format changes
		expectedMinPrefixes int
	}{
		{
			name:                "gitlab",
			provider:            NewGitLabProvider(),
			expectedMinPrefixes: 2,
		},
		{
			name:                "hetzner",
			provider:            NewHetznerProvider(),
			expectedMinPrefixes: 30,
		},
		// Uncomment these to test against real APIs
		// They are commented by default to avoid rate limits and external dependencies
		/*
			{
				name:                "github",
				provider:            NewGitHubProvider(""),
				expectedMinPrefixes: 10,
			},
			{
				name:                "github-hooks",
				provider:            NewGitHubProvider("hooks"),
				expectedMinPrefixes: 1,
			},
			{
				name:                "cloudflare-ipv4",
				provider:            NewCloudflareProvider(false),
				expectedMinPrefixes: 10,
			},
			{
				name:                "google",
				provider:            NewGoogleProvider(),
				expectedMinPrefixes: 100,
			},
			{
				name:                "atlassian",
				provider:            NewAtlassianProvider(),
				expectedMinPrefixes: 10,
			},
			{
				name:                "aws",
				provider:            NewAWSProvider("", ""),
				expectedMinPrefixes: 100,
			},
		*/
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefixes, err := tt.provider.Prefixes(ctx)
			require.NoError(t, err, "Failed to fetch prefixes from %s", tt.name)
			assert.GreaterOrEqual(t, len(prefixes), tt.expectedMinPrefixes,
				"Expected at least %d prefixes from %s, got %d",
				tt.expectedMinPrefixes, tt.name, len(prefixes))

			// Verify all returned values are valid Prefix objects
			for i, prefix := range prefixes {
				assert.True(t, prefix.IsValid(), "Prefix %d is invalid", i)
			}

			t.Logf("%s returned %d prefixes", tt.name, len(prefixes))
		})
	}
}

// TestProviderResponseFormat tests that providers return valid CIDR ranges
func TestProviderResponseFormat(t *testing.T) {
	tests := []struct {
		name          string
		provider      Provider
		testIP        string
		shouldContain bool
	}{
		{
			name:          "gitlab webhook IP in range",
			provider:      NewGitLabProvider(),
			testIP:        "34.74.90.65",
			shouldContain: true,
		},
		{
			name:          "gitlab webhook IP not in range",
			provider:      NewGitLabProvider(),
			testIP:        "1.2.3.4",
			shouldContain: false,
		},
		{
			name:          "hetzner IP in range",
			provider:      NewHetznerProvider(),
			testIP:        "5.9.1.1",
			shouldContain: true,
		},
		{
			name:          "hetzner IP not in range",
			provider:      NewHetznerProvider(),
			testIP:        "1.2.3.4",
			shouldContain: false,
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefixes, err := tt.provider.Prefixes(ctx)
			require.NoError(t, err)

			addr, err := netip.ParseAddr(tt.testIP)
			require.NoError(t, err)

			found := false
			for _, prefix := range prefixes {
				if prefix.Contains(addr) {
					found = true
					break
				}
			}

			assert.Equal(t, tt.shouldContain, found,
				"IP %s should%s be in %s ranges",
				tt.testIP,
				map[bool]string{true: "", false: " not"}[tt.shouldContain],
				tt.provider.Name())
		})
	}
}
