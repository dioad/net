package prefixlist

import (
	"time"
)

func init() {
	RegisterProvider("cloudflare", func(cfg ProviderConfig) (Provider, error) {
		// Cloudflare: support "version" key (e.g., "ipv6")
		version := cfg.Filter["version"]
		return NewCloudflareProvider(version == "ipv6"), nil
	})
}

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
