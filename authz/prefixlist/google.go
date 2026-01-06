package prefixlist

import (
	"net/netip"
	"strings"
	"time"
)

// GoogleProvider fetches IP ranges from Google Cloud
type GoogleProvider struct {
	*HTTPJSONProvider[googleIPRanges]
	scopes   []string // optional filter for scopes (e.g., "us-central1", "europe-west1")
	services []string // optional filter for services (e.g., "Google Cloud", "Google Cloud Storage")
}

type googleIPRanges struct {
	Prefixes []struct {
		IPv4Prefix string `json:"ipv4Prefix,omitempty"`
		IPv6Prefix string `json:"ipv6Prefix,omitempty"`
		Service    string `json:"service,omitempty"`
		Scope      string `json:"scope,omitempty"`
	} `json:"prefixes"`
}

// NewGoogleProvider creates a new Google Cloud prefix list provider
// scopes: optional list of scopes to filter by (e.g., ["us-central1", "europe-west1"])
// services: optional list of services to filter by (e.g., ["Google Cloud"])
func NewGoogleProvider(scopes, services []string) *GoogleProvider {
	name := "google"
	if len(services) > 0 {
		name += "-" + strings.Join(services, ",")
	}
	if len(scopes) > 0 {
		name += "-" + strings.Join(scopes, ",")
	}
	
	p := &GoogleProvider{
		scopes:   scopes,
		services: services,
	}
	
	p.HTTPJSONProvider = NewHTTPJSONProvider[googleIPRanges](
		name,
		"https://www.gstatic.com/ipranges/cloud.json",
		CacheConfig{
			StaticExpiry: 24 * time.Hour,
			ReturnStale:  true,
		},
		p.transformGoogleRanges,
	)
	
	return p
}

func (p *GoogleProvider) transformGoogleRanges(data googleIPRanges) ([]netip.Prefix, error) {
	var cidrs []string
	for _, prefix := range data.Prefixes {
		// Apply scope filter if specified
		if len(p.scopes) > 0 && !contains(p.scopes, prefix.Scope) {
			continue
		}

		// Apply service filter if specified
		if len(p.services) > 0 && !contains(p.services, prefix.Service) {
			continue
		}

		if prefix.IPv4Prefix != "" {
			cidrs = append(cidrs, prefix.IPv4Prefix)
		}
		if prefix.IPv6Prefix != "" {
			cidrs = append(cidrs, prefix.IPv6Prefix)
		}
	}

	return parseCIDRs(cidrs)
}

// contains checks if a slice contains a string (case-insensitive)
func contains(slice []string, item string) bool {
	itemLower := strings.ToLower(item)
	for _, s := range slice {
		if strings.ToLower(s) == itemLower {
			return true
		}
	}
	return false
}
