package prefixlist

import (
	"net/netip"
	"time"
)

func init() {
	RegisterProvider("fastly", func(cfg ProviderConfig) (Provider, error) {
		return NewFastlyProvider(), nil
	})
}

// FastlyProvider fetches IP ranges from Fastly CDN
type FastlyProvider struct {
	*HTTPJSONProvider[fastlyIPRanges]
}

type fastlyIPRanges struct {
	Addresses     []string `json:"addresses"`
	IPv6Addresses []string `json:"ipv6_addresses"`
}

// NewFastlyProvider creates a new Fastly prefix list provider
func NewFastlyProvider() *FastlyProvider {
	p := &FastlyProvider{}

	p.HTTPJSONProvider = NewHTTPJSONProvider[fastlyIPRanges](
		"fastly",
		"https://api.fastly.com/public-ip-list",
		CacheConfig{
			StaticExpiry: 24 * time.Hour,
			ReturnStale:  true,
		},
		transformFastlyRanges,
	)

	return p
}

func transformFastlyRanges(data fastlyIPRanges) ([]netip.Prefix, error) {
	var cidrs []string
	cidrs = append(cidrs, data.Addresses...)
	cidrs = append(cidrs, data.IPv6Addresses...)

	return parseCIDRs(cidrs)
}
