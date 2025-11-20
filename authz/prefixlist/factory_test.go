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
			name: "github with filter map",
			config: ProviderConfig{
				Name:    "github",
				Enabled: true,
				Filter:  map[string]string{"service": "hooks"},
			},
			wantName: "github-hooks",
			wantErr:  false,
		},
		{
			name: "github with filter string (backward compat)",
			config: ProviderConfig{
				Name:         "github",
				Enabled:      true,
				FilterString: "hooks",
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
			name: "cloudflare ipv6 with map",
			config: ProviderConfig{
				Name:    "cloudflare",
				Enabled: true,
				Filter:  map[string]string{"version": "ipv6"},
			},
			wantName: "cloudflare-ipv6",
			wantErr:  false,
		},
		{
			name: "cloudflare ipv6 with string (backward compat)",
			config: ProviderConfig{
				Name:         "cloudflare",
				Enabled:      true,
				FilterString: "ipv6",
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
			name: "aws with filter map",
			config: ProviderConfig{
				Name:    "aws",
				Enabled: true,
				Filter:  map[string]string{"service": "EC2", "region": "us-east-1"},
			},
			wantName: "aws-EC2-us-east-1",
			wantErr:  false,
		},
		{
			name: "aws with filter string (backward compat)",
			config: ProviderConfig{
				Name:         "aws",
				Enabled:      true,
				FilterString: "EC2:us-east-1",
			},
			wantName: "aws-EC2-us-east-1",
			wantErr:  false,
		},
		{
			name: "google with filter map",
			config: ProviderConfig{
				Name:    "google",
				Enabled: true,
				Filter:  map[string]string{"scope": "us-central1", "service": "Google Cloud"},
			},
			wantName: "google-Google Cloud-us-central1",
			wantErr:  false,
		},
		{
			name: "atlassian with filter map",
			config: ProviderConfig{
				Name:    "atlassian",
				Enabled: true,
				Filter:  map[string]string{"region": "global", "product": "jira"},
			},
			wantName: "atlassian-jira-global",
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
					Filter:  map[string]string{"service": "hooks"},
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

func TestParseFilterStringToMap(t *testing.T) {
	tests := []struct {
		name         string
		providerName string
		filter       string
		want         map[string]string
	}{
		{
			name:         "github filter",
			providerName: "github",
			filter:       "hooks",
			want:         map[string]string{"service": "hooks"},
		},
		{
			name:         "aws service only",
			providerName: "aws",
			filter:       "EC2",
			want:         map[string]string{"service": "EC2"},
		},
		{
			name:         "aws service and region",
			providerName: "aws",
			filter:       "EC2:us-east-1",
			want:         map[string]string{"service": "EC2", "region": "us-east-1"},
		},
		{
			name:         "google scope and service",
			providerName: "google",
			filter:       "us-central1:Google Cloud",
			want:         map[string]string{"scope": "us-central1", "service": "Google Cloud"},
		},
		{
			name:         "google scope only",
			providerName: "google",
			filter:       "us-central1",
			want:         map[string]string{"scope": "us-central1"},
		},
		{
			name:         "atlassian region and product",
			providerName: "atlassian",
			filter:       "global:jira",
			want:         map[string]string{"region": "global", "product": "jira"},
		},
		{
			name:         "cloudflare ipv6",
			providerName: "cloudflare",
			filter:       "ipv6",
			want:         map[string]string{"version": "ipv6"},
		},
		{
			name:         "empty filter",
			providerName: "github",
			filter:       "",
			want:         map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFilterStringToMap(tt.providerName, tt.filter)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseCommaSeparated(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{
			name:  "single value",
			value: "us-central1",
			want:  []string{"us-central1"},
		},
		{
			name:  "multiple values",
			value: "us-central1,europe-west1",
			want:  []string{"us-central1", "europe-west1"},
		},
		{
			name:  "values with spaces",
			value: "us-central1, europe-west1 , asia-east1",
			want:  []string{"us-central1", "europe-west1", "asia-east1"},
		},
		{
			name:  "empty string",
			value: "",
			want:  nil,
		},
		{
			name:  "only commas",
			value: ",,,",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCommaSeparated(tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}
