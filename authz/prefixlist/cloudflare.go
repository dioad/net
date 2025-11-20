package prefixlist

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

// CloudflareProvider fetches IP ranges from Cloudflare
type CloudflareProvider struct {
	ipv6 bool
}

// NewCloudflareProvider creates a new Cloudflare prefix list provider
func NewCloudflareProvider(ipv6 bool) *CloudflareProvider {
	return &CloudflareProvider{
		ipv6: ipv6,
	}
}

func (p *CloudflareProvider) Name() string {
	if p.ipv6 {
		return "cloudflare-ipv6"
	}
	return "cloudflare-ipv4"
}

func (p *CloudflareProvider) FetchPrefixes(ctx context.Context) ([]netip.Prefix, error) {
	url := "https://www.cloudflare.com/ips-v4/"
	if p.ipv6 {
		url = "https://www.cloudflare.com/ips-v6/"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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

	return parseTextPrefixes(resp.Body)
}

// parseTextPrefixes parses plain text list of CIDR ranges (one per line)
func parseTextPrefixes(r io.Reader) ([]netip.Prefix, error) {
	var cidrs []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			cidrs = append(cidrs, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return parseCIDRs(cidrs)
}
