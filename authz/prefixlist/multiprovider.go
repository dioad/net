package prefixlist

import (
	"context"
	"fmt"
	"net/netip"
	"sync"

	"github.com/rs/zerolog"
)

// MultiProvider wraps multiple providers and implements the Provider interface
type MultiProvider struct {
	providers []Provider
	prefixes  []netip.Prefix
	mu        sync.RWMutex
	logger    zerolog.Logger
}

// NewMultiProvider creates a new multi-provider that wraps multiple providers
func NewMultiProvider(providers []Provider, logger zerolog.Logger) *MultiProvider {
	return &MultiProvider{
		providers: providers,
		prefixes:  []netip.Prefix{},
		logger:    logger,
	}
}

// Name returns a combined name of all providers
func (m *MultiProvider) Name() string {
	if len(m.providers) == 0 {
		return "multiprovider-empty"
	}
	if len(m.providers) == 1 {
		return m.providers[0].Name()
	}
	return fmt.Sprintf("multiprovider-%d-providers", len(m.providers))
}

// FetchPrefixes fetches prefixes from all wrapped providers
func (m *MultiProvider) FetchPrefixes(ctx context.Context) ([]netip.Prefix, error) {
	var allPrefixes []netip.Prefix
	var fetchErrors []error

	for _, provider := range m.providers {
		prefixes, err := provider.FetchPrefixes(ctx)
		if err != nil {
			m.logger.Error().
				Err(err).
				Str("provider", provider.Name()).
				Msg("failed to fetch prefixes")
			fetchErrors = append(fetchErrors, fmt.Errorf("%s: %w", provider.Name(), err))
			continue
		}

		m.logger.Debug().
			Str("provider", provider.Name()).
			Int("count", len(prefixes)).
			Msg("fetched prefixes")

		allPrefixes = append(allPrefixes, prefixes...)
	}

	// Cache the result
	m.mu.Lock()
	m.prefixes = allPrefixes
	m.mu.Unlock()

	if len(fetchErrors) > 0 && len(allPrefixes) == 0 {
		return nil, fmt.Errorf("all providers failed: %v", fetchErrors)
	}

	return allPrefixes, nil
}

// Contains checks if an IP address is in any of the cached prefix lists
func (m *MultiProvider) Contains(addr netip.Addr) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, prefix := range m.prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// GetPrefixes returns a copy of all current prefixes
func (m *MultiProvider) GetPrefixes() []netip.Prefix {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]netip.Prefix, len(m.prefixes))
	copy(result, m.prefixes)
	return result
}
