package prefixlist

// Config represents the configuration for prefix list providers
type Config struct {
	// Providers lists the enabled providers
	Providers []ProviderConfig `mapstructure:"providers" yaml:"providers"`
}

// ProviderConfig represents configuration for a single provider
type ProviderConfig struct {
	// Name is the provider name (github, cloudflare, google, atlassian, gitlab, aws)
	Name string `mapstructure:"name" yaml:"name"`

	// Enabled controls whether this provider is active
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`

	// Filter optionally filters prefixes using a map of key-value pairs
	// Examples:
	//   GitHub: {"service": "hooks"} or {"service": "actions"}
	//   AWS: {"service": "EC2", "region": "us-east-1"}
	//   Google: {"scope": "us-central1", "service": "Google Cloud"}
	//   Atlassian: {"region": "global", "product": "jira"}
	//   Cloudflare: {"version": "ipv6"}
	Filter map[string]string `mapstructure:"filter" yaml:"filter,omitempty"`
}
