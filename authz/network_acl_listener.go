package authz

import (
	"net"

	"github.com/rs/zerolog"
)

type Listener struct {
	NetworkACL *NetworkACL
	Listener   net.Listener
	Logger     zerolog.Logger
}

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

func (l *Listener) Close() error {
	return l.Listener.Close()
}

func (l *Listener) Addr() net.Addr {
	return l.Listener.Addr()
}
