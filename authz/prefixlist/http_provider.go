package prefixlist

import (
	"context"
	"net/netip"

	"github.com/dioad/net/httpcache"
)

// TransformFunc is a function that transforms fetched data into a list of prefixes.
type TransformFunc[T any] func(T) ([]netip.Prefix, error)

// HTTPJSONProvider is a generic provider that fetches JSON data and transforms it into prefixes.
type HTTPJSONProvider[T any] struct {
	name      string
	fetcher   *httpcache.CachingFetcher[T]
	transform TransformFunc[T]
}

// NewHTTPJSONProvider creates a new HTTP JSON-based provider
// Parameters:
//   - name: the name of the provider (e.g., "github", "aws")
//   - url: the HTTP endpoint to fetch from
//   - config: caching configuration
//   - transform: function to transform the JSON response into prefixes
func NewHTTPJSONProvider[T any](name, url string, config httpcache.CacheConfig, transform TransformFunc[T]) *HTTPJSONProvider[T] {
	return &HTTPJSONProvider[T]{
		name:      name,
		fetcher:   httpcache.NewCachingFetcher[T](url, config),
		transform: transform,
	}
}

// Name returns the provider's name.
func (p *HTTPJSONProvider[T]) Name() string {
	return p.name
}

// Prefixes fetches (or returns cached) JSON data from the provider's
// endpoint and transforms it into a list of prefixes.
func (p *HTTPJSONProvider[T]) Prefixes(ctx context.Context) ([]netip.Prefix, error) {
	data, _, err := p.fetcher.Get(ctx)
	if err != nil {
		return nil, err
	}

	return p.transform(data)
}

// Contains reports whether addr is contained in any of the provider's
// prefixes.
func (p *HTTPJSONProvider[T]) Contains(addr netip.Addr) bool {
	prefixes, err := p.Prefixes(context.Background())
	if err != nil {
		return false
	}
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}

// HTTPTextProvider is a provider for HTTP endpoints that return plain text lists of prefixes.
type HTTPTextProvider struct {
	name    string
	fetcher *httpcache.CachingFetcher[[]string]
}

// NewHTTPTextProvider creates a new HTTP text-based provider
// The endpoint is expected to return a plain text list of CIDR ranges (one per line).
func NewHTTPTextProvider(name, url string, config httpcache.CacheConfig) *HTTPTextProvider {
	return &HTTPTextProvider{
		name: name,
		fetcher: httpcache.NewCachingFetcherWithFunc[[]string](
			url,
			config,
			httpcache.FetchTextLines,
		),
	}
}

// Name returns the provider's name.
func (p *HTTPTextProvider) Name() string {
	return p.name
}

// Prefixes fetches (or returns cached) text data from the provider's
// endpoint and parses it into a list of prefixes.
func (p *HTTPTextProvider) Prefixes(ctx context.Context) ([]netip.Prefix, error) {
	cidrs, _, err := p.fetcher.Get(ctx)
	if err != nil {
		return nil, err
	}

	return parseCIDRs(cidrs)
}

// Contains reports whether addr is contained in any of the provider's
// prefixes.
func (p *HTTPTextProvider) Contains(addr netip.Addr) bool {
	prefixes, err := p.Prefixes(context.Background())
	if err != nil {
		return false
	}
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}
