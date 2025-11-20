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
		// Parse filter for scope and service (format: "scope1,scope2:service1,service2")
		scopes, services := parseGoogleFilter(cfg.Filter)
		return NewGoogleProvider(scopes, services), nil
	case "atlassian":
		// Parse filter for region and product (format: "region1,region2:product1,product2")
		regions, products := parseAtlassianFilter(cfg.Filter)
		return NewAtlassianProvider(regions, products), nil
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

// parseGoogleFilter parses Google filter string in format "scope1,scope2:service1,service2"
// Returns scopes and services as separate slices
func parseGoogleFilter(filter string) (scopes, services []string) {
	if filter == "" {
		return nil, nil
	}

	parts := strings.SplitN(filter, ":", 2)
	if len(parts) > 0 && parts[0] != "" {
		scopes = strings.Split(parts[0], ",")
		for i := range scopes {
			scopes[i] = strings.TrimSpace(scopes[i])
		}
	}
	if len(parts) > 1 && parts[1] != "" {
		services = strings.Split(parts[1], ",")
		for i := range services {
			services[i] = strings.TrimSpace(services[i])
		}
	}
	return
}

// parseAtlassianFilter parses Atlassian filter string in format "region1,region2:product1,product2"
// Returns regions and products as separate slices
func parseAtlassianFilter(filter string) (regions, products []string) {
	if filter == "" {
		return nil, nil
	}

	parts := strings.SplitN(filter, ":", 2)
	if len(parts) > 0 && parts[0] != "" {
		regions = strings.Split(parts[0], ",")
		for i := range regions {
			regions[i] = strings.TrimSpace(regions[i])
		}
	}
	if len(parts) > 1 && parts[1] != "" {
		products = strings.Split(parts[1], ",")
		for i := range products {
			products[i] = strings.TrimSpace(products[i])
		}
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
