package authz

import (
	"net"

	"github.com/rs/zerolog"
)

// Listener is a network listener that enforces a NetworkACL on all incoming connections.
type Listener struct {
	NetworkACL *NetworkACL
	Listener   net.Listener
	Logger     zerolog.Logger
}

// Accept waits for and returns the next connection to the listener.
// It checks each connection against the NetworkACL and closes it if not authorised.
func (l *Listener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	authorised, err := l.NetworkACL.AuthoriseConn(c)
	if err != nil {
		return nil, err
	}

	if !authorised {
		l.Logger.Warn().Stringer("remoteAddr", c.RemoteAddr()).Msg("access denied")
		err = c.Close()
		if err != nil {
			l.Logger.Error().Err(err).Msg("closeConnError")
		}
	}

	return c, nil
}

// Close closes the listener.
func (l *Listener) Close() error {
	return l.Listener.Close()
}

// Addr returns the listener's network address.
func (l *Listener) Addr() net.Addr {
	return l.Listener.Addr()
}
