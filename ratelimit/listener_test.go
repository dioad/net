package ratelimit

import (
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListener_Accept(t *testing.T) {
	// Create a real TCP listener on localhost
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	// Create a rate limiter: 1 RPS, burst of 2
	rl := NewRateLimiter(1.0, 2, zerolog.Nop())
	rl.CleanupInterval = 100 * time.Millisecond // fast cleanup for testing

	rlListener := NewListener(ln, rl, zerolog.Nop())

	// Start a goroutine to accept connections
	var acceptedCount int32 // use int32 so we can safely update it with atomic.AddInt32 / atomic.LoadInt32 across goroutines
	
	go func() {
		for {
			conn, err := rlListener.Accept()
			if err != nil {
				return
			}
			atomic.AddInt32(&acceptedCount, 1)

			conn.Close()
		}
	}()

	// Try to connect multiple times
	dialer := &net.Dialer{Timeout: 100 * time.Millisecond}

	// First two should be allowed (burst = 2)
	for i := 0; i < 2; i++ {
		conn, err := dialer.Dial("tcp", ln.Addr().String())
		assert.NoError(t, err)
		if err == nil {
			// Read to see if it's closed by server or still open
			// Actually, the listener closes it AFTER accepting if it's rate limited.
			// If it's NOT rate limited, it returns it and we close it in the goroutine.
			// If it IS rate limited, rlListener.Accept() won't return it to our goroutine,
			// it will close it and continue to the next Accept().
			conn.Close()
		}
	}

	var acceptedCountTmp int32

	// Give it a moment for the goroutine to process
	time.Sleep(50 * time.Millisecond)
	acceptedCountTmp = atomic.LoadInt32(&acceptedCount)
	assert.Equal(t, 2, int(acceptedCountTmp))

	// Third one should be rate limited and closed immediately by rlListener.Accept()
	// It won't reach our acceptedCount++
	conn, err := dialer.Dial("tcp", ln.Addr().String())
	assert.NoError(t, err)
	if err == nil {
		// We expect the server to close this connection because of rate limiting
		buf := make([]byte, 1)
		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		_, err := conn.Read(buf)
		// Should get EOF or similar because server closed it
		assert.Error(t, err)
		conn.Close()
	}

	time.Sleep(50 * time.Millisecond)
	acceptedCountTmp = atomic.LoadInt32(&acceptedCount)
	assert.Equal(t, 2, int(acceptedCountTmp)) // Still 2

	// Wait for refill (1 sec)
	time.Sleep(1100 * time.Millisecond)

	// Fourth one should be allowed now
	conn, err = dialer.Dial("tcp", ln.Addr().String())
	assert.NoError(t, err)
	if err == nil {
		conn.Close()
	}

	time.Sleep(50 * time.Millisecond)
	acceptedCountTmp = atomic.LoadInt32(&acceptedCount)
	assert.Equal(t, 3, int(acceptedCountTmp))
}

func TestListener_getPrincipal(t *testing.T) {
	rl := NewRateLimiter(1.0, 1, zerolog.Nop())
	l := NewListener(nil, rl, zerolog.Nop())

	tests := []struct {
		name       string
		network    string
		remoteAddr string
		expected   string
	}{
		{
			name:       "IPv4 with port",
			network:    "tcp",
			remoteAddr: "192.168.1.1:12345",
			expected:   "192.168.1.1",
		},
		{
			name:       "IPv6 with port",
			network:    "tcp6",
			remoteAddr: "[::1]:12345",
			expected:   "::1",
		},
		{
			name:       "Unix socket",
			network:    "unix",
			remoteAddr: "/tmp/test.sock",
			expected:   "/tmp/test.sock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock connection
			mockConn := &mockAddrConn{addr: tt.remoteAddr, network: tt.network}
			assert.Equal(t, tt.expected, l.getPrincipal(mockConn))
		})
	}
}

type mockAddrConn struct {
	net.Conn
	network string
	addr    string
}

func (m *mockAddrConn) RemoteAddr() net.Addr {
	return &mockAddr{addr: m.addr, network: m.network}
}

type mockAddr struct {
	network string
	addr    string
}

func (m *mockAddr) Network() string { return m.network }
func (m *mockAddr) String() string  { return m.addr }
