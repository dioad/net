// Package metrics provides utilities for tracking network connection and listener metrics.
package metrics

import (
	"net"

	"github.com/rs/zerolog"
)

type ListenerMetrics interface {
	AcceptedCount() int
	ResetMetrics()
}

// Listener wraps a net.Listener and tracks accepted connection metrics.
type Listener struct {
	ln            net.Listener
	acceptedCount int
	logger        zerolog.Logger
	useLogger     bool
}

func (l *Listener) ResetMetrics() {
	l.acceptedCount = 0
}

func (l *Listener) AcceptedCount() int {
	return l.acceptedCount
}

func (l *Listener) Accept() (net.Conn, error) {
	conn, err := l.ln.Accept()

	if err != nil {
		return nil, err
	}

	l.acceptedCount += 1
	var connWithMetrics net.Conn
	if l.useLogger {
		connWithMetrics = NewConnWithLogger(conn, l.logger)
	} else {
		connWithMetrics = NewConn(conn)
	}

	return connWithMetrics, err
}

func (l *Listener) Close() error {
	return l.ln.Close()
}

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
