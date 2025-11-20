package prefixlist

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// NewProviderFromConfig creates a provider instance from configuration
func NewProviderFromConfig(cfg ProviderConfig) (Provider, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("provider %s is not enabled", cfg.Name)
	}

	name := strings.ToLower(cfg.Name)
	
	// Get filter as map, handling backward compatibility
	filterMap := getFilterMap(cfg)

	switch name {
	case "github":
		// GitHub: support "service" key (e.g., "hooks", "actions")
		service := filterMap["service"]
		return NewGitHubProvider(service), nil
	case "cloudflare":
		// Cloudflare: support "version" key (e.g., "ipv6")
		version := filterMap["version"]
		return NewCloudflareProvider(version == "ipv6"), nil
	case "google":
		// Google: support "scope" and "service" keys (comma-separated values)
		scopes := parseCommaSeparated(filterMap["scope"])
		services := parseCommaSeparated(filterMap["service"])
		return NewGoogleProvider(scopes, services), nil
	case "atlassian":
		// Atlassian: support "region" and "product" keys (comma-separated values)
		regions := parseCommaSeparated(filterMap["region"])
		products := parseCommaSeparated(filterMap["product"])
		return NewAtlassianProvider(regions, products), nil
	case "gitlab":
		return NewGitLabProvider(), nil
	case "aws":
		// AWS: support "service" and "region" keys
		service := filterMap["service"]
		region := filterMap["region"]
		return NewAWSProvider(service, region), nil
	case "fastly":
		return NewFastlyProvider(), nil
	case "hetzner":
		return NewHetznerProvider(), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Name)
	}
}

// getFilterMap converts the filter configuration to a map, handling backward compatibility
func getFilterMap(cfg ProviderConfig) map[string]string {
	// If Filter map is provided, use it directly
	if len(cfg.Filter) > 0 {
		return cfg.Filter
	}
	
	// For backward compatibility, convert FilterString to map based on provider
	if cfg.FilterString != "" {
		return parseFilterStringToMap(cfg.Name, cfg.FilterString)
	}
	
	return make(map[string]string)
}

// parseFilterStringToMap converts old string-based filters to map format for backward compatibility
func parseFilterStringToMap(providerName, filter string) map[string]string {
	if filter == "" {
		return make(map[string]string)
	}
	
	result := make(map[string]string)
	name := strings.ToLower(providerName)
	
	switch name {
	case "github":
		// GitHub: simple service name
		result["service"] = filter
	case "cloudflare":
		// Cloudflare: "ipv6" or empty
		if filter == "ipv6" {
			result["version"] = "ipv6"
		}
	case "google":
		// Google: "scope:service" format
		scopes, services := parseColonSeparatedPair(filter)
		if scopes != "" {
			result["scope"] = scopes
		}
		if services != "" {
			result["service"] = services
		}
	case "atlassian":
		// Atlassian: "region:product" format
		regions, products := parseColonSeparatedPair(filter)
		if regions != "" {
			result["region"] = regions
		}
		if products != "" {
			result["product"] = products
		}
	case "aws":
		// AWS: "service:region" format
		service, region := parseColonSeparatedPair(filter)
		if service != "" {
			result["service"] = service
		}
		if region != "" {
			result["region"] = region
		}
	}
	
	return result
}

// parseColonSeparatedPair parses "first:second" format
func parseColonSeparatedPair(filter string) (first, second string) {
	parts := strings.SplitN(filter, ":", 2)
	first = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		second = strings.TrimSpace(parts[1])
	}
	return
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

// NewManagerFromConfig creates a manager from configuration
func NewManagerFromConfig(cfg Config, logger zerolog.Logger) (*Manager, error) {
	var providers []Provider

	for _, providerCfg := range cfg.Providers {
		provider, err := NewProviderFromConfig(providerCfg)
		if err != nil {
			logger.Warn().Err(err).Str("provider", providerCfg.Name).Msg("failed to create provider")
			continue
		}

		// Override cache duration if specified in config
		if providerCfg.CacheDuration > 0 {
			provider = &cacheDurationOverride{
				Provider: provider,
				duration: providerCfg.CacheDuration,
			}
		}

		providers = append(providers, provider)
	}

	if len(providers) == 0 {
		return nil, fmt.Errorf("no valid providers configured")
	}

	return NewManager(providers, logger), nil
}

// cacheDurationOverride wraps a provider to override its cache duration
type cacheDurationOverride struct {
	Provider
	duration time.Duration
}

func (c *cacheDurationOverride) CacheDuration() time.Duration {
	return c.duration
}
