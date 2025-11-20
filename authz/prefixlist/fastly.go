package prefixlist

import (
	"context"
	"encoding/json"
	"net"
	"time"
)

// FastlyProvider fetches IP ranges from Fastly CDN
type FastlyProvider struct{}

type fastlyIPRanges struct {
	Addresses     []string `json:"addresses"`
	IPv6Addresses []string `json:"ipv6_addresses"`
}

// NewFastlyProvider creates a new Fastly prefix list provider
func NewFastlyProvider() *FastlyProvider {
	return &FastlyProvider{}
}

func (p *FastlyProvider) Name() string {
	return "fastly"
}

func (p *FastlyProvider) CacheDuration() time.Duration {
	return 24 * time.Hour
}

func (p *FastlyProvider) FetchPrefixes(ctx context.Context) ([]*net.IPNet, error) {
	body, err := fetchURL(ctx, "https://api.fastly.com/public-ip-list")
	if err != nil {
		return nil, err
	}

	var data fastlyIPRanges
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	var cidrs []string
	cidrs = append(cidrs, data.Addresses...)
	cidrs = append(cidrs, data.IPv6Addresses...)

	return parseCIDRs(cidrs)
}
