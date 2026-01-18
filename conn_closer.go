package net

import (
	"net"
	"time"
)

// RawConn is an interface for getting the underlying net.Conn from a wrapped connection.
type RawConn interface {
	NetConn() net.Conn
}

type connWithCloser struct {
	conn    DoneConn
	onClose func(net.Conn)
}

func (s *connWithCloser) NetConn() net.Conn {
	return s.conn
}

func (s *connWithCloser) Read(b []byte) (int, error) {
	return s.conn.Read(b)
}

func (s *connWithCloser) Write(b []byte) (int, error) {
	return s.conn.Write(b)
}

func (s *connWithCloser) Close() error {
	if !s.conn.Closed() {
		if s.onClose != nil {
			s.onClose(s.conn)
		}
		return s.conn.Close()
	}
	return net.ErrClosed
}

func (s *connWithCloser) Done() <-chan struct{} {
	return s.conn.Done()
}

func (s *connWithCloser) Closed() bool {
	return s.conn.Closed()
}

func (s *connWithCloser) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

func (s *connWithCloser) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

func (s *connWithCloser) SetDeadline(t time.Time) error {
	return s.conn.SetDeadline(t)
}

func (s *connWithCloser) SetReadDeadline(t time.Time) error {
	return s.conn.SetReadDeadline(t)
}

func (s *connWithCloser) SetWriteDeadline(t time.Time) error {
	return s.conn.SetWriteDeadline(t)
}

// NewConnWithCloser wraps a net.Conn with a function that is called when the connection is closed.
func NewConnWithCloser(c net.Conn, closer func(net.Conn)) DoneConn {
	return &connWithCloser{
		conn:    NewDoneConn(c),
		onClose: closer,
	}
}
