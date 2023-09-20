package metrics

import (
	"net"

	"github.com/rs/zerolog"
)

type ListenerMetrics interface {
	AcceptedCount() int
	ResetMetrics()
}

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

func NewListener(l net.Listener) *Listener {
	return &Listener{
		ln: l,
	}
}

func NewListenerWithLogger(l net.Listener, logger zerolog.Logger) *Listener {
	return &Listener{
		ln:        l,
		logger:    logger,
		useLogger: true,
	}
}
