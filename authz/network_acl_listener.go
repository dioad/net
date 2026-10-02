package authz

import (
	"errors"
	"net"

	"github.com/rs/zerolog"
)

// Listener is a network listener that enforces an Authoriser on all incoming connections.
type Listener struct {
	acl    Authoriser
	Logger zerolog.Logger
	inner  *GatingListener
}

// NewListener creates a Listener that gates incoming connections via the given Authoriser.
func NewListener(l net.Listener, acl Authoriser, logger zerolog.Logger) *Listener {
	ln := &Listener{acl: acl, Logger: logger}
	ln.inner = NewGatingListener(l, ln.gate)

	return ln
}

func (l *Listener) gate(c net.Conn) bool {
	err := l.acl.AuthoriseConn(c)
	if err == nil {
		return true
	}

	if errors.Is(err, ErrDenied) {
		l.Logger.Warn().Err(err).Stringer("remote_addr", c.RemoteAddr()).Msg("network ACL denied connection")
		return false
	}

	// Not a deliberate denial - the ACL couldn't evaluate the address at
	// all (e.g. RemoteAddr didn't parse as host:port), which is a more
	// unusual condition worth a higher severity.
	l.Logger.Error().Err(err).Stringer("remote_addr", c.RemoteAddr()).Msg("network ACL check failed")

	return false
}

// Accept waits for and returns the next connection that passes the Authoriser.
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
