package prefixlist

import (
	"context"
	"net/netip"
	"time"
)

// FastlyProvider fetches IP ranges from Fastly CDN
type FastlyProvider struct {
	fetcher *CachingFetcher[fastlyIPRanges]
}

type fastlyIPRanges struct {
	Addresses     []string `json:"addresses"`
	IPv6Addresses []string `json:"ipv6_addresses"`
}

// NewFastlyProvider creates a new Fastly prefix list provider
func NewFastlyProvider() *FastlyProvider {
	return &FastlyProvider{
		fetcher: NewCachingFetcher[fastlyIPRanges](
			"https://api.fastly.com/public-ip-list",
			CacheConfig{
				StaticExpiry: 24 * time.Hour,
				ReturnStale:  true,
			},
		),
	}
}

func (p *FastlyProvider) Name() string {
	return "fastly"
}

func (p *FastlyProvider) FetchPrefixes(ctx context.Context) ([]netip.Prefix, error) {
	data, _, err := p.fetcher.Get(ctx)
	if err != nil {
		return nil, err
	}

	var cidrs []string
	cidrs = append(cidrs, data.Addresses...)
	cidrs = append(cidrs, data.IPv6Addresses...)

	return parseCIDRs(cidrs)
}
