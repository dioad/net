package prefixlist

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// FastlyProvider fetches IP ranges from Fastly CDN
type FastlyProvider struct{}

type fastlyIPRanges struct {
	Addresses    []string `json:"addresses"`
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
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.fastly.com/public-ip-list", nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
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
