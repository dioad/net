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

// AWSProvider fetches IP ranges from AWS
type AWSProvider struct {
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
	return &AWSProvider{
		service: service,
		region:  region,
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

func (p *AWSProvider) CacheDuration() time.Duration {
	return 24 * time.Hour
}

func (p *AWSProvider) FetchPrefixes(ctx context.Context) ([]*net.IPNet, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://ip-ranges.amazonaws.com/ip-ranges.json", nil)
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

	var data awsIPRanges
	if err := json.Unmarshal(body, &data); err != nil {
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
