// Package metrics provides utilities for tracking network connection and listener metrics.
package metrics

import (
	"net"
	"sync/atomic"

	"github.com/rs/zerolog"
)

// ListenerMetrics exposes accepted-connection counting for a listener.
type ListenerMetrics interface {
	AcceptedCount() int
	ResetMetrics()
}

// Listener wraps a net.Listener and tracks accepted connection metrics.
type Listener struct {
	ln            net.Listener
	acceptedCount atomic.Int64
	logger        zerolog.Logger
	useLogger     bool
}

// ResetMetrics resets the accepted-connection count to zero.
func (l *Listener) ResetMetrics() {
	l.acceptedCount.Store(0)
}

// AcceptedCount returns the number of connections accepted so far.
func (l *Listener) AcceptedCount() int {
	return int(l.acceptedCount.Load())
}

// Accept waits for and returns the next connection, wrapped to track its
// lifetime metrics.
func (l *Listener) Accept() (net.Conn, error) {
	conn, err := l.ln.Accept()

	if err != nil {
		return nil, err
	}

	l.acceptedCount.Add(1)
	var connWithMetrics net.Conn
	if l.useLogger {
		connWithMetrics = NewConnWithLogger(conn, l.logger)
	} else {
		connWithMetrics = NewConn(conn)
	}

	return connWithMetrics, err
}

// Close closes the underlying listener.
func (l *Listener) Close() error {
	return l.ln.Close()
}

// Addr returns the underlying listener's network address.
func (l *Listener) Addr() net.Addr {
	return l.ln.Addr()
}

// NewListener creates a new Listener wrapping the provided net.Listener.
func NewListener(l net.Listener) *Listener {
	return &Listener{
		ln: l,
	}
}

// NewListenerWithLogger creates a new Listener wrapping the provided net.Listener and logs metrics using the provided logger.
func NewListenerWithLogger(l net.Listener, logger zerolog.Logger) *Listener {
	return &Listener{
		ln:        l,
		logger:    logger,
		useLogger: true,
	}
}
