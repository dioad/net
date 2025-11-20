package prefixlist

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
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

func (p *CloudflareProvider) CacheDuration() time.Duration {
	return 24 * time.Hour
}

func (p *CloudflareProvider) FetchPrefixes(ctx context.Context) ([]*net.IPNet, error) {
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
func parseTextPrefixes(r io.Reader) ([]*net.IPNet, error) {
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
