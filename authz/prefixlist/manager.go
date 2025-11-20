package prefixlist

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Manager handles multiple prefix list providers with caching and periodic updates
type Manager struct {
	providers   []Provider
	prefixes    []*net.IPNet
	mu          sync.RWMutex
	logger      zerolog.Logger
	stopCh      chan struct{}
	updateTicker *time.Ticker
}

// NewManager creates a new prefix list manager with the given providers
func NewManager(providers []Provider, logger zerolog.Logger) *Manager {
	return &Manager{
		providers: providers,
		prefixes:  []*net.IPNet{},
		logger:    logger,
		stopCh:    make(chan struct{}),
	}
}

// Start initializes the manager and starts periodic updates
func (m *Manager) Start(ctx context.Context) error {
	// Initial fetch
	if err := m.Update(ctx); err != nil {
		return fmt.Errorf("initial update failed: %w", err)
	}

	// Determine update interval (use minimum cache duration from providers)
	var minDuration time.Duration
	for _, p := range m.providers {
		d := p.CacheDuration()
		if minDuration == 0 || d < minDuration {
			minDuration = d
		}
	}
	
	if minDuration == 0 {
		minDuration = 1 * time.Hour // default
	}

	// Start periodic updates
	m.updateTicker = time.NewTicker(minDuration)
	go m.updateLoop(ctx)

	m.logger.Info().
		Int("providers", len(m.providers)).
		Dur("update_interval", minDuration).
		Msg("prefix list manager started")

	return nil
}

// Stop stops the periodic updates
func (m *Manager) Stop() {
	if m.updateTicker != nil {
		m.updateTicker.Stop()
	}
	close(m.stopCh)
}

// Update fetches fresh prefix lists from all providers
func (m *Manager) Update(ctx context.Context) error {
	var allPrefixes []*net.IPNet
	var updateErrors []error

	for _, provider := range m.providers {
		prefixes, err := provider.FetchPrefixes(ctx)
		if err != nil {
			m.logger.Error().
				Err(err).
				Str("provider", provider.Name()).
				Msg("failed to fetch prefixes")
			updateErrors = append(updateErrors, fmt.Errorf("%s: %w", provider.Name(), err))
			continue
		}

		m.logger.Debug().
			Str("provider", provider.Name()).
			Int("count", len(prefixes)).
			Msg("fetched prefixes")

		allPrefixes = append(allPrefixes, prefixes...)
	}

	m.mu.Lock()
	m.prefixes = allPrefixes
	m.mu.Unlock()

	if len(updateErrors) > 0 && len(allPrefixes) == 0 {
		return fmt.Errorf("all providers failed: %v", updateErrors)
	}

	return nil
}

// Contains checks if an IP address is in any of the managed prefix lists
func (m *Manager) Contains(ip net.IP) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, prefix := range m.prefixes {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

// GetPrefixes returns a copy of all current prefixes
func (m *Manager) GetPrefixes() []*net.IPNet {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*net.IPNet, len(m.prefixes))
	copy(result, m.prefixes)
	return result
}

func (m *Manager) updateLoop(ctx context.Context) {
	for {
		select {
		case <-m.stopCh:
			return
		case <-m.updateTicker.C:
			if err := m.Update(ctx); err != nil {
				m.logger.Warn().Err(err).Msg("periodic update failed")
			}
		case <-ctx.Done():
			return
		}
	}
}
