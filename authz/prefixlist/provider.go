package prefixlist

import (
	"context"
	"net/netip"
)

// Provider defines the interface for fetching IP prefix lists from different sources
type Provider interface {
	// Name returns the provider name (e.g., "github", "cloudflare")
	Name() string
	
	// FetchPrefixes fetches the current list of IP prefixes from the provider
	FetchPrefixes(ctx context.Context) ([]netip.Prefix, error)
}
