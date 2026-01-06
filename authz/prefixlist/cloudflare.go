package prefixlist

import (
	"time"
)

// CloudflareProvider fetches IP ranges from Cloudflare
type CloudflareProvider struct {
	*HTTPTextProvider
}

// NewCloudflareProvider creates a new Cloudflare prefix list provider
func NewCloudflareProvider(ipv6 bool) *CloudflareProvider {
	name := "cloudflare-ipv4"
	url := "https://www.cloudflare.com/ips-v4/"
	if ipv6 {
		name = "cloudflare-ipv6"
		url = "https://www.cloudflare.com/ips-v6/"
	}
	
	return &CloudflareProvider{
		HTTPTextProvider: NewHTTPTextProvider(
			name,
			url,
			CacheConfig{
				StaticExpiry: 24 * time.Hour,
				ReturnStale:  true,
			},
		),
	}
}
