package prefixlist

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider is a test provider
type mockProvider struct {
	name     string
	prefixes []string
	duration time.Duration
	fetchErr error
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) CacheDuration() time.Duration {
	return m.duration
}

func (m *mockProvider) FetchPrefixes(ctx context.Context) ([]*net.IPNet, error) {
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	return parseCIDRs(m.prefixes)
}

func TestManager(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("basic functionality", func(t *testing.T) {
		provider := &mockProvider{
			name:     "test",
			prefixes: []string{"192.168.1.0/24", "10.0.0.0/8"},
			duration: 1 * time.Hour,
		}

		manager := NewManager([]Provider{provider}, logger)
		ctx := context.Background()

		err := manager.Start(ctx)
		require.NoError(t, err)
		defer manager.Stop()

		// Test Contains
		tests := []struct {
			ip       string
			expected bool
		}{
			{"192.168.1.1", true},
			{"192.168.1.255", true},
			{"10.1.2.3", true},
			{"172.16.0.1", false},
			{"8.8.8.8", false},
		}

		for _, tt := range tests {
			ip := net.ParseIP(tt.ip)
			assert.Equal(t, tt.expected, manager.Contains(ip), "IP: %s", tt.ip)
		}
	})

	t.Run("multiple providers", func(t *testing.T) {
		provider1 := &mockProvider{
			name:     "test1",
			prefixes: []string{"192.168.0.0/16"},
			duration: 1 * time.Hour,
		}
		provider2 := &mockProvider{
			name:     "test2",
			prefixes: []string{"10.0.0.0/8"},
			duration: 1 * time.Hour,
		}

		manager := NewManager([]Provider{provider1, provider2}, logger)
		ctx := context.Background()

		err := manager.Start(ctx)
		require.NoError(t, err)
		defer manager.Stop()

		// Should contain IPs from both providers
		assert.True(t, manager.Contains(net.ParseIP("192.168.1.1")))
		assert.True(t, manager.Contains(net.ParseIP("10.1.2.3")))
		assert.False(t, manager.Contains(net.ParseIP("172.16.0.1")))
	})

	t.Run("get prefixes", func(t *testing.T) {
		provider := &mockProvider{
			name:     "test",
			prefixes: []string{"192.168.1.0/24"},
			duration: 1 * time.Hour,
		}

		manager := NewManager([]Provider{provider}, logger)
		ctx := context.Background()

		err := manager.Start(ctx)
		require.NoError(t, err)
		defer manager.Stop()

		prefixes := manager.GetPrefixes()
		assert.Len(t, prefixes, 1)
	})

	t.Run("update", func(t *testing.T) {
		provider := &mockProvider{
			name:     "test",
			prefixes: []string{"192.168.1.0/24"},
			duration: 1 * time.Hour,
		}

		manager := NewManager([]Provider{provider}, logger)
		ctx := context.Background()

		err := manager.Start(ctx)
		require.NoError(t, err)
		defer manager.Stop()

		// Initial state
		assert.True(t, manager.Contains(net.ParseIP("192.168.1.1")))
		assert.False(t, manager.Contains(net.ParseIP("10.0.0.1")))

		// Update provider prefixes
		provider.prefixes = []string{"10.0.0.0/8"}

		// Trigger update
		err = manager.Update(ctx)
		require.NoError(t, err)

		// Verify new state
		assert.False(t, manager.Contains(net.ParseIP("192.168.1.1")))
		assert.True(t, manager.Contains(net.ParseIP("10.0.0.1")))
	})
}
