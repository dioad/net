package prefixlist

import (
	"testing"

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
				Filter:  map[string]string{"service": "hooks"},
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
				Filter:  map[string]string{"version": "ipv6"},
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
			name: "google with filter",
			config: ProviderConfig{
				Name:    "google",
				Enabled: true,
				Filter:  map[string]string{"scope": "us-central1", "service": "Google Cloud"},
			},
			wantName: "google-Google Cloud-us-central1",
			wantErr:  false,
		},
		{
			name: "atlassian with filter",
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

func TestNewMultiProviderFromConfig(t *testing.T) {
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

		multiProvider, err := NewMultiProviderFromConfig(config, logger)
		require.NoError(t, err)
		assert.NotNil(t, multiProvider)
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

		multiProvider, err := NewMultiProviderFromConfig(config, logger)
		require.NoError(t, err)
		assert.NotNil(t, multiProvider)
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

		_, err := NewMultiProviderFromConfig(config, logger)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no valid providers")
	})
}
