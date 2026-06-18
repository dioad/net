package authz

import (
	"net"

	"github.com/rs/zerolog"
)

// Listener is a network listener that enforces a NetworkACL on all incoming connections.
type Listener struct {
	NetworkACL *NetworkACL
	Logger     zerolog.Logger
	inner      *GatingListener
}

// NewListener creates a Listener that gates incoming connections via the given NetworkACL.
func NewListener(l net.Listener, acl *NetworkACL, logger zerolog.Logger) *Listener {
	ln := &Listener{NetworkACL: acl, Logger: logger}
	ln.inner = NewGatingListener(l, ln.gate)
	return ln
}

func (l *Listener) gate(c net.Conn) (bool, error) {
	allowed, err := l.NetworkACL.AuthoriseConn(c)
	if err != nil {
		return false, err
	}
	if !allowed {
		l.Logger.Warn().Stringer("remoteAddr", c.RemoteAddr()).Msg("access denied")
	}
	return allowed, nil
}

// Accept waits for and returns the next connection that passes the NetworkACL.
// Rejected connections are closed; the loop retries until an authorised connection arrives.
func (l *Listener) Accept() (net.Conn, error) {
	return l.inner.Accept()
}

// Close closes the listener.
func (l *Listener) Close() error {
	return l.inner.Close()
}

// Addr returns the listener's network address.
func (l *Listener) Addr() net.Addr {
	return l.inner.Addr()
}
