package prefixlist

import (
	"context"
	"net/netip"
)

// HetznerProvider provides static IP ranges for Hetzner Cloud
type HetznerProvider struct{}

// NewHetznerProvider creates a new Hetzner prefix list provider
func NewHetznerProvider() *HetznerProvider {
	return &HetznerProvider{}
}

func (p *HetznerProvider) Name() string {
	return "hetzner"
}

func (p *HetznerProvider) FetchPrefixes(ctx context.Context) ([]netip.Prefix, error) {
	// Hetzner Cloud main IP ranges
	// These are well-known stable ranges for Hetzner services
	cidrs := []string{
		// Hetzner Cloud IPv4
		"5.9.0.0/16",
		"23.88.0.0/16",
		"46.4.0.0/16",
		"49.12.0.0/16",
		"65.108.0.0/16",
		"78.46.0.0/16",
		"78.47.0.0/16",
		"88.198.0.0/16",
		"88.99.0.0/16",
		"91.107.128.0/17",
		"94.130.0.0/16",
		"95.216.0.0/16",
		"116.202.0.0/16",
		"135.181.0.0/16",
		"138.201.0.0/16",
		"142.132.128.0/17",
		"144.76.0.0/16",
		"148.251.0.0/16",
		"157.90.0.0/16",
		"159.69.0.0/16",
		"161.97.0.0/16",
		"162.55.0.0/16",
		"167.233.0.0/16",
		"167.235.0.0/16",
		"168.119.0.0/16",
		"176.9.0.0/16",
		"178.63.0.0/16",
		"188.34.128.0/17",
		"188.40.0.0/16",
		"195.201.0.0/16",
		"213.133.96.0/19",
		"213.239.192.0/18",
		
		// Hetzner Cloud IPv6
		"2a01:4f8::/32",
		"2a01:4f9::/32",
	}

	return parseCIDRs(cidrs)
}
