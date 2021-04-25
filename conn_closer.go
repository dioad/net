package net

import (
	"net"
	"time"
)

type ConnWithCloser struct {
	conn    net.Conn
	onClose func(net.Conn)
}

func (s *ConnWithCloser) RawConn() net.Conn {
	return s.conn
}

func (s *ConnWithCloser) Read(b []byte) (int, error) {
	return s.conn.Read(b)
}

func (s *ConnWithCloser) Write(b []byte) (int, error) {
	return s.conn.Write(b)
}

func (s *ConnWithCloser) Close() error {
	if s.onClose != nil {
		s.onClose(s.conn)
	}
	return s.conn.Close()
}

func (s *ConnWithCloser) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

func (s *ConnWithCloser) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

func (s *ConnWithCloser) SetDeadline(t time.Time) error {
	return s.conn.SetDeadline(t)
}

func (s *ConnWithCloser) SetReadDeadline(t time.Time) error {
	return s.conn.SetReadDeadline(t)
}

func (s *ConnWithCloser) SetWriteDeadline(t time.Time) error {
	return s.conn.SetWriteDeadline(t)
}

//func NewConnWithCloser(c net.Conn, closer func ()) *ConnWithCloser {
func NewConnWithCloser(c net.Conn, closer func(net.Conn)) net.Conn {
	return &ConnWithCloser{
		conn:    c,
		onClose: closer,
	}
}
