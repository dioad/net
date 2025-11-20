package prefixlist

import (
	"time"
)

// Config represents the configuration for prefix list providers
type Config struct {
	// Providers lists the enabled providers
	Providers []ProviderConfig `mapstructure:"providers" yaml:"providers"`
	
	// UpdateInterval specifies how often to refresh prefix lists (optional, defaults to provider-specific duration)
	UpdateInterval time.Duration `mapstructure:"update_interval" yaml:"update_interval"`
}

// ProviderConfig represents configuration for a single provider
type ProviderConfig struct {
	// Name is the provider name (github, cloudflare, google, atlassian, gitlab, aws)
	Name string `mapstructure:"name" yaml:"name"`
	
	// Enabled controls whether this provider is active
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`
	
	// CacheDuration overrides the default cache duration for this provider
	CacheDuration time.Duration `mapstructure:"cache_duration" yaml:"cache_duration,omitempty"`
	
	// Filter optionally filters prefixes (e.g., "hooks" for GitHub webhooks only)
	Filter string `mapstructure:"filter" yaml:"filter,omitempty"`
}
