package prefixlist

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProviderFromConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   ProviderConfig
		wantName string
		wantErr  bool
	}{
		{
			name: "github enabled",
			config: ProviderConfig{
				Name:    "github",
				Enabled: true,
			},
			wantName: "github",
			wantErr:  false,
		},
		{
			name: "github with filter",
			config: ProviderConfig{
				Name:    "github",
				Enabled: true,
				Filter:  "hooks",
			},
			wantName: "github-hooks",
			wantErr:  false,
		},
		{
			name: "cloudflare ipv4",
			config: ProviderConfig{
				Name:    "cloudflare",
				Enabled: true,
			},
			wantName: "cloudflare-ipv4",
			wantErr:  false,
		},
		{
			name: "cloudflare ipv6",
			config: ProviderConfig{
				Name:    "cloudflare",
				Enabled: true,
				Filter:  "ipv6",
			},
			wantName: "cloudflare-ipv6",
			wantErr:  false,
		},
		{
			name: "google",
			config: ProviderConfig{
				Name:    "google",
				Enabled: true,
			},
			wantName: "google",
			wantErr:  false,
		},
		{
			name: "atlassian",
			config: ProviderConfig{
				Name:    "atlassian",
				Enabled: true,
			},
			wantName: "atlassian",
			wantErr:  false,
		},
		{
			name: "gitlab",
			config: ProviderConfig{
				Name:    "gitlab",
				Enabled: true,
			},
			wantName: "gitlab",
			wantErr:  false,
		},
		{
			name: "aws",
			config: ProviderConfig{
				Name:    "aws",
				Enabled: true,
			},
			wantName: "aws",
			wantErr:  false,
		},
		{
			name: "aws with filter",
			config: ProviderConfig{
				Name:    "aws",
				Enabled: true,
				Filter:  "EC2:us-east-1",
			},
			wantName: "aws-EC2-us-east-1",
			wantErr:  false,
		},
		{
			name: "fastly",
			config: ProviderConfig{
				Name:    "fastly",
				Enabled: true,
			},
			wantName: "fastly",
			wantErr:  false,
		},
		{
			name: "hetzner",
			config: ProviderConfig{
				Name:    "hetzner",
				Enabled: true,
			},
			wantName: "hetzner",
			wantErr:  false,
		},
		{
			name: "disabled provider",
			config: ProviderConfig{
				Name:    "github",
				Enabled: false,
			},
			wantErr: true,
		},
		{
			name: "unknown provider",
			config: ProviderConfig{
				Name:    "unknown",
				Enabled: true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewProviderFromConfig(tt.config)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, provider.Name())
		})
	}
}

func TestCacheDurationOverride(t *testing.T) {
	baseProvider := NewGitHubProvider("")
	originalDuration := baseProvider.CacheDuration()

	override := &cacheDurationOverride{
		Provider: baseProvider,
		duration: 5 * time.Hour,
	}

	assert.Equal(t, originalDuration, 1*time.Hour)
	assert.Equal(t, 5*time.Hour, override.CacheDuration())
	assert.Equal(t, "github", override.Name())
}

func TestNewManagerFromConfig(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("valid config", func(t *testing.T) {
		config := Config{
			Providers: []ProviderConfig{
				{
					Name:    "gitlab",
					Enabled: true,
				},
			},
		}

		manager, err := NewManagerFromConfig(config, logger)
		require.NoError(t, err)
		assert.NotNil(t, manager)
	})

	t.Run("multiple providers", func(t *testing.T) {
		config := Config{
			Providers: []ProviderConfig{
				{
					Name:    "gitlab",
					Enabled: true,
				},
				{
					Name:    "github",
					Enabled: true,
					Filter:  "hooks",
				},
			},
		}

		manager, err := NewManagerFromConfig(config, logger)
		require.NoError(t, err)
		assert.NotNil(t, manager)
	})

	t.Run("no valid providers", func(t *testing.T) {
		config := Config{
			Providers: []ProviderConfig{
				{
					Name:    "github",
					Enabled: false,
				},
			},
		}

		_, err := NewManagerFromConfig(config, logger)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no valid providers")
	})

	t.Run("with cache duration override", func(t *testing.T) {
		config := Config{
			Providers: []ProviderConfig{
				{
					Name:          "gitlab",
					Enabled:       true,
					CacheDuration: 10 * time.Hour,
				},
			},
		}

		manager, err := NewManagerFromConfig(config, logger)
		require.NoError(t, err)
		assert.NotNil(t, manager)
	})
}

func TestParseGoogleFilter(t *testing.T) {
	tests := []struct {
		name         string
		filter       string
		wantScopes   []string
		wantServices []string
	}{
		{
			name:         "empty filter",
			filter:       "",
			wantScopes:   nil,
			wantServices: nil,
		},
		{
			name:         "scope only",
			filter:       "us-central1",
			wantScopes:   []string{"us-central1"},
			wantServices: nil,
		},
		{
			name:         "multiple scopes",
			filter:       "us-central1,europe-west1",
			wantScopes:   []string{"us-central1", "europe-west1"},
			wantServices: nil,
		},
		{
			name:         "scope and service",
			filter:       "us-central1:Google Cloud",
			wantScopes:   []string{"us-central1"},
			wantServices: []string{"Google Cloud"},
		},
		{
			name:         "multiple scopes and services",
			filter:       "us-central1,europe-west1:Google Cloud,Google Cloud Storage",
			wantScopes:   []string{"us-central1", "europe-west1"},
			wantServices: []string{"Google Cloud", "Google Cloud Storage"},
		},
		{
			name:         "service only",
			filter:       ":Google Cloud",
			wantScopes:   nil,
			wantServices: []string{"Google Cloud"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scopes, services := parseGoogleFilter(tt.filter)
			assert.Equal(t, tt.wantScopes, scopes)
			assert.Equal(t, tt.wantServices, services)
		})
	}
}

func TestParseAtlassianFilter(t *testing.T) {
	tests := []struct {
		name         string
		filter       string
		wantRegions  []string
		wantProducts []string
	}{
		{
			name:         "empty filter",
			filter:       "",
			wantRegions:  nil,
			wantProducts: nil,
		},
		{
			name:         "region only",
			filter:       "global",
			wantRegions:  []string{"global"},
			wantProducts: nil,
		},
		{
			name:         "multiple regions",
			filter:       "global,us-east-1",
			wantRegions:  []string{"global", "us-east-1"},
			wantProducts: nil,
		},
		{
			name:         "region and product",
			filter:       "global:jira",
			wantRegions:  []string{"global"},
			wantProducts: []string{"jira"},
		},
		{
			name:         "multiple regions and products",
			filter:       "global,us-east-1:jira,confluence",
			wantRegions:  []string{"global", "us-east-1"},
			wantProducts: []string{"jira", "confluence"},
		},
		{
			name:         "product only",
			filter:       ":jira",
			wantRegions:  nil,
			wantProducts: []string{"jira"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regions, products := parseAtlassianFilter(tt.filter)
			assert.Equal(t, tt.wantRegions, regions)
			assert.Equal(t, tt.wantProducts, products)
		})
	}
}
