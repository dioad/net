package prefixlist

import (
	"context"
	"net"
	"net/netip"
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
	fetchErr error
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) FetchPrefixes(ctx context.Context) ([]netip.Prefix, error) {
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	return parseCIDRs(m.prefixes)
}

func TestListener(t *testing.T) {
	logger := zerolog.Nop()

	// Create a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	// Create a multi-provider with test provider
	provider := &mockProvider{
		name:     "test",
		prefixes: []string{"127.0.0.0/8"}, // Allow localhost
	}

	multiProvider := NewMultiProvider([]Provider{provider}, logger)
	ctx := context.Background()
	_, err = multiProvider.FetchPrefixes(ctx)
	require.NoError(t, err)

	// Create prefix list listener
	plListener := NewListener(listener, multiProvider, logger)

	// Test accepting a connection from allowed IP
	go func() {
		conn, err := net.Dial("tcp", listener.Addr().String())
		if err == nil {
			defer conn.Close()
			time.Sleep(100 * time.Millisecond)
		}
	}()

	acceptedConn, err := plListener.Accept()
	require.NoError(t, err)
	assert.NotNil(t, acceptedConn)
	acceptedConn.Close()
}

func TestListenerAddr(t *testing.T) {
	logger := zerolog.Nop()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	provider := &mockProvider{
		name:     "test",
		prefixes: []string{"127.0.0.0/8"},
	}

	multiProvider := NewMultiProvider([]Provider{provider}, logger)
	plListener := NewListener(listener, multiProvider, logger)

	assert.Equal(t, listener.Addr(), plListener.Addr())
}
