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

// AtlassianProvider fetches IP ranges from Atlassian
type AtlassianProvider struct{}

type atlassianIPRanges struct {
	Items []struct {
		CIDR string `json:"cidr"`
	} `json:"items"`
}

// NewAtlassianProvider creates a new Atlassian prefix list provider
func NewAtlassianProvider() *AtlassianProvider {
	return &AtlassianProvider{}
}

func (p *AtlassianProvider) Name() string {
	return "atlassian"
}

func (p *AtlassianProvider) CacheDuration() time.Duration {
	return 24 * time.Hour
}

func (p *AtlassianProvider) FetchPrefixes(ctx context.Context) ([]*net.IPNet, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://ip-ranges.atlassian.com/", nil)
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

	var data atlassianIPRanges
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	var cidrs []string
	for _, item := range data.Items {
		cidrs = append(cidrs, item.CIDR)
	}

	return parseCIDRs(cidrs)
}
