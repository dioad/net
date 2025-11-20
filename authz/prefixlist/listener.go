package prefixlist

import (
	"net"

	"github.com/rs/zerolog"
)

// Listener wraps a net.Listener and filters connections based on prefix lists
type Listener struct {
	listener net.Listener
	manager  *Manager
	logger   zerolog.Logger
}

// NewListener creates a new prefix list filtering listener
func NewListener(listener net.Listener, manager *Manager, logger zerolog.Logger) *Listener {
	return &Listener{
		listener: listener,
		manager:  manager,
		logger:   logger,
	}
}

// Accept waits for and returns the next connection, filtering based on prefix lists
func (l *Listener) Accept() (net.Conn, error) {
	for {
		conn, err := l.listener.Accept()
		if err != nil {
			return nil, err
		}

		// Extract IP from remote address
		tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
		if !ok {
			l.logger.Warn().
				Str("remoteAddr", conn.RemoteAddr().String()).
				Msg("non-TCP connection, rejecting")
			conn.Close()
			continue
		}

		// Check if IP is in allowed prefix lists
		if !l.manager.Contains(tcpAddr.IP) {
			l.logger.Warn().
				Str("remoteAddr", tcpAddr.IP.String()).
				Msg("connection not in allowed prefix lists, rejecting")
			conn.Close()
			continue
		}

		l.logger.Debug().
			Str("remoteAddr", tcpAddr.IP.String()).
			Msg("connection accepted")

		return conn, nil
	}
}

// Close closes the underlying listener
func (l *Listener) Close() error {
	return l.listener.Close()
}

// Addr returns the listener's network address
func (l *Listener) Addr() net.Addr {
	return l.listener.Addr()
}
