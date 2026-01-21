package prefixlist

import (
	"net/netip"
	"time"
)

func init() {
	RegisterProvider("aws", func(cfg ProviderConfig) (Provider, error) {
		// AWS: support "service" and "region" keys
		service := cfg.Filter["service"]
		region := cfg.Filter["region"]
		return NewAWSProvider(service, region), nil
	})
}

// AWSProvider fetches IP ranges from AWS
type AWSProvider struct {
	*HTTPJSONProvider[awsIPRanges]
	service string // optional filter for specific service (e.g., "CLOUDFRONT", "EC2")
	region  string // optional filter for specific region (e.g., "us-east-1")
}

type awsIPRanges struct {
	Prefixes []struct {
		IPPrefix string `json:"ip_prefix"`
		Region   string `json:"region"`
		Service  string `json:"service"`
	} `json:"prefixes"`
	IPv6Prefixes []struct {
		IPv6Prefix string `json:"ipv6_prefix"`
		Region     string `json:"region"`
		Service    string `json:"service"`
	} `json:"ipv6_prefixes"`
}

// NewAWSProvider creates a new AWS prefix list provider
func NewAWSProvider(service, region string) *AWSProvider {
	name := "aws"
	if service != "" {
		name += "-" + service
	}
	if region != "" {
		name += "-" + region
	}

	p := &AWSProvider{
		service: service,
		region:  region,
	}

	p.HTTPJSONProvider = NewHTTPJSONProvider[awsIPRanges](
		name,
		"https://ip-ranges.amazonaws.com/ip-ranges.json",
		CacheConfig{
			StaticExpiry: 24 * time.Hour,
			ReturnStale:  true,
		},
		p.transformAWSRanges,
	)

	return p
}

func (p *AWSProvider) transformAWSRanges(data awsIPRanges) ([]netip.Prefix, error) {
	var cidrs []string

	// Filter IPv4 prefixes
	for _, prefix := range data.Prefixes {
		if p.matchesFilter(prefix.Service, prefix.Region) {
			cidrs = append(cidrs, prefix.IPPrefix)
		}
	}

	// Filter IPv6 prefixes
	for _, prefix := range data.IPv6Prefixes {
		if p.matchesFilter(prefix.Service, prefix.Region) {
			cidrs = append(cidrs, prefix.IPv6Prefix)
		}
	}

	return parseCIDRs(cidrs)
}

func (p *AWSProvider) matchesFilter(service, region string) bool {
	if p.service != "" && service != p.service {
		return false
	}
	if p.region != "" && region != p.region {
		return false
	}
	return true
}
