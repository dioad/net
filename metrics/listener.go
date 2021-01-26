package metrics

import (
	"net"
)

type Listener struct {
	ln            net.Listener
	acceptedCount int
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
	connWithMetrics := NewConn(conn)

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
