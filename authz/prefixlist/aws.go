package prefixlist

import (
	"context"
	"net/netip"
	"time"
)

// AWSProvider fetches IP ranges from AWS
type AWSProvider struct {
	service string // optional filter for specific service (e.g., "CLOUDFRONT", "EC2")
	region  string // optional filter for specific region (e.g., "us-east-1")
	fetcher *CachingFetcher[awsIPRanges]
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
	return &AWSProvider{
		service: service,
		region:  region,
		fetcher: NewCachingFetcher[awsIPRanges](
			"https://ip-ranges.amazonaws.com/ip-ranges.json",
			CacheConfig{
				StaticExpiry: 24 * time.Hour,
				ReturnStale:  true,
			},
		),
	}
}

func (p *AWSProvider) Name() string {
	name := "aws"
	if p.service != "" {
		name += "-" + p.service
	}
	if p.region != "" {
		name += "-" + p.region
	}
	return name
}

func (p *AWSProvider) FetchPrefixes(ctx context.Context) ([]netip.Prefix, error) {
	data, _, err := p.fetcher.Get(ctx)
	if err != nil {
		return nil, err
	}

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
