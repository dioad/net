package prefixlist

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"
)

// NewProviderFromConfig creates a provider instance from configuration
func NewProviderFromConfig(cfg ProviderConfig) (Provider, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("provider %s is not enabled", cfg.Name)
	}

	name := strings.ToLower(cfg.Name)

	switch name {
	case "github":
		// GitHub: support "service" key (e.g., "hooks", "actions")
		service := cfg.Filter["service"]
		return NewGitHubProvider(service), nil
	case "cloudflare":
		// Cloudflare: support "version" key (e.g., "ipv6")
		version := cfg.Filter["version"]
		return NewCloudflareProvider(version == "ipv6"), nil
	case "google":
		// Google: support "scope" and "service" keys (comma-separated values)
		scopes := parseCommaSeparated(cfg.Filter["scope"])
		services := parseCommaSeparated(cfg.Filter["service"])
		return NewGoogleProvider(scopes, services), nil
	case "atlassian":
		// Atlassian: support "region" and "product" keys (comma-separated values)
		regions := parseCommaSeparated(cfg.Filter["region"])
		products := parseCommaSeparated(cfg.Filter["product"])
		return NewAtlassianProvider(regions, products), nil
	case "gitlab":
		return NewGitLabProvider(), nil
	case "aws":
		// AWS: support "service" and "region" keys
		service := cfg.Filter["service"]
		region := cfg.Filter["region"]
		return NewAWSProvider(service, region), nil
	case "fastly":
		return NewFastlyProvider(), nil
	case "hetzner":
		return NewHetznerProvider(), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Name)
	}
}

// parseCommaSeparated parses comma-separated values into a slice
func parseCommaSeparated(value string) []string {
	if value == "" {
		return nil
	}
	
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	
	if len(result) == 0 {
		return nil
	}
	return result
}

// NewMultiProviderFromConfig creates a MultiProvider from configuration
func NewMultiProviderFromConfig(cfg Config, logger zerolog.Logger) (*MultiProvider, error) {
	var providers []Provider

	for _, providerCfg := range cfg.Providers {
		provider, err := NewProviderFromConfig(providerCfg)
		if err != nil {
			logger.Warn().Err(err).Str("provider", providerCfg.Name).Msg("failed to create provider")
			continue
		}

		providers = append(providers, provider)
	}

	if len(providers) == 0 {
		return nil, fmt.Errorf("no valid providers configured")
	}

	return NewMultiProvider(providers, logger), nil
}
