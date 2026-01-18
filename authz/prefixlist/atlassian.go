package prefixlist

import (
	"net/netip"
	"strings"
	"time"
)

// AtlassianProvider fetches IP ranges from Atlassian
type AtlassianProvider struct {
	*HTTPJSONProvider[atlassianIPRanges]
	regions  []string // optional filter for regions (e.g., ["us-east-1", "global"])
	products []string // optional filter for products (e.g., ["jira", "confluence"])
}

type atlassianIPRanges struct {
	Items []struct {
		CIDR      string   `json:"cidr"`
		Region    []string `json:"region"`
		Product   []string `json:"product"`
		Direction []string `json:"direction"`
	} `json:"items"`
}

// NewAtlassianProvider creates a new Atlassian prefix list provider
// regions: optional list of regions to filter by (e.g., ["global", "us-east-1"])
// products: optional list of products to filter by (e.g., ["jira", "confluence"])
// Only prefixes with "egress" direction are included
func NewAtlassianProvider(regions, products []string) *AtlassianProvider {
	name := "atlassian"
	if len(products) > 0 {
		name += "-" + strings.Join(products, ",")
	}
	if len(regions) > 0 {
		name += "-" + strings.Join(regions, ",")
	}

	p := &AtlassianProvider{
		regions:  regions,
		products: products,
	}

	p.HTTPJSONProvider = NewHTTPJSONProvider[atlassianIPRanges](
		name,
		"https://ip-ranges.atlassian.com/",
		CacheConfig{
			StaticExpiry: 24 * time.Hour,
			ReturnStale:  true,
		},
		p.transformAtlassianRanges,
	)

	return p
}

func (p *AtlassianProvider) transformAtlassianRanges(data atlassianIPRanges) ([]netip.Prefix, error) {
	var cidrs []string
	for _, item := range data.Items {
		// Only include egress direction
		if !containsAny(item.Direction, []string{"egress"}) {
			continue
		}

		// Apply region filter if specified
		if len(p.regions) > 0 && !containsAny(item.Region, p.regions) {
			continue
		}

		// Apply product filter if specified
		if len(p.products) > 0 && !containsAny(item.Product, p.products) {
			continue
		}

		cidrs = append(cidrs, item.CIDR)
	}

	return parseCIDRs(cidrs)
}

// containsAny checks if any item from needles exists in haystack (case-insensitive)
func containsAny(haystack, needles []string) bool {
	if len(needles) == 0 {
		return true
	}
	for _, needle := range needles {
		if contains(haystack, needle) {
			return true
		}
	}
	return false
}
