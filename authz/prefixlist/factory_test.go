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
