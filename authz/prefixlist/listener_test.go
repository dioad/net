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

func TestListener(t *testing.T) {
	logger := zerolog.Nop()

	// Create a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	// Create a manager with test provider
	provider := &mockProvider{
		name:     "test",
		prefixes: []string{"127.0.0.0/8"}, // Allow localhost
		duration: 1 * time.Hour,
	}

	manager := NewManager([]Provider{provider}, logger)
	ctx := context.Background()
	err = manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop()

	// Create prefix list listener
	plListener := NewListener(listener, manager, logger)

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
		duration: 1 * time.Hour,
	}

	manager := NewManager([]Provider{provider}, logger)
	plListener := NewListener(listener, manager, logger)

	assert.Equal(t, listener.Addr(), plListener.Addr())
}
