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

	switch name {
	case "github":
		return NewGitHubProvider(cfg.Filter), nil
	case "cloudflare":
		// Default to IPv4, can be extended with filter for "ipv6"
		return NewCloudflareProvider(cfg.Filter == "ipv6"), nil
	case "google":
		return NewGoogleProvider(), nil
	case "atlassian":
		return NewAtlassianProvider(), nil
	case "gitlab":
		return NewGitLabProvider(), nil
	case "aws":
		// Parse filter for service and region (format: "service:region" or just "service")
		service, region := parseAWSFilter(cfg.Filter)
		return NewAWSProvider(service, region), nil
	case "fastly":
		return NewFastlyProvider(), nil
	case "hetzner":
		return NewHetznerProvider(), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Name)
	}
}

// parseAWSFilter parses AWS filter string in format "service:region" or just "service"
func parseAWSFilter(filter string) (service, region string) {
	if filter == "" {
		return "", ""
	}

	parts := strings.SplitN(filter, ":", 2)
	service = parts[0]
	if len(parts) > 1 {
		region = parts[1]
	}
	return
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
