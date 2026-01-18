// Package prefixlist provides utilities for fetching and managing IP prefix lists from various cloud providers.
package prefixlist

import (
	"context"
	"net/netip"
)

// Provider defines the interface for fetching IP prefix lists from different sources
type Provider interface {
	// Name returns the provider name (e.g., "github", "cloudflare")
	Name() string

	// Prefixes returns the current list of IP prefixes from the provider
	Prefixes(ctx context.Context) ([]netip.Prefix, error)

	// Contains checks if an IP address is in the provider's prefix list
	Contains(addr netip.Addr) bool
}
