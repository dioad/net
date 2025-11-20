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

// GoogleProvider fetches IP ranges from Google Cloud
type GoogleProvider struct{}

type googleIPRanges struct {
	Prefixes []struct {
		IPv4Prefix string `json:"ipv4Prefix,omitempty"`
		IPv6Prefix string `json:"ipv6Prefix,omitempty"`
	} `json:"prefixes"`
}

// NewGoogleProvider creates a new Google Cloud prefix list provider
func NewGoogleProvider() *GoogleProvider {
	return &GoogleProvider{}
}

func (p *GoogleProvider) Name() string {
	return "google"
}

func (p *GoogleProvider) CacheDuration() time.Duration {
	return 24 * time.Hour
}

func (p *GoogleProvider) FetchPrefixes(ctx context.Context) ([]*net.IPNet, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.gstatic.com/ipranges/cloud.json", nil)
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

	var data googleIPRanges
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	var cidrs []string
	for _, prefix := range data.Prefixes {
		if prefix.IPv4Prefix != "" {
			cidrs = append(cidrs, prefix.IPv4Prefix)
		}
		if prefix.IPv6Prefix != "" {
			cidrs = append(cidrs, prefix.IPv6Prefix)
		}
	}

	return parseCIDRs(cidrs)
}
