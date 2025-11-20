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
	
	// Filter optionally filters prefixes using a map of key-value pairs
	// Examples:
	//   GitHub: {"service": "hooks"} or {"service": "actions"}
	//   AWS: {"service": "EC2", "region": "us-east-1"}
	//   Google: {"scope": "us-central1", "service": "Google Cloud"}
	//   Atlassian: {"region": "global", "product": "jira"}
	//   Cloudflare: {"version": "ipv6"}
	// For backward compatibility, Filter is also supported as a string
	Filter map[string]string `mapstructure:"filter" yaml:"filter,omitempty"`
	
	// FilterString is deprecated but maintained for backward compatibility
	// Use Filter (map) instead for clearer semantics
	FilterString string `mapstructure:"filter_string" yaml:"filter_string,omitempty"`
}
