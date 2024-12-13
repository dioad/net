package net

import (
	"net"
	"sync"
	"time"
)

type DoneConn interface {
	net.Conn
	RawConn
	Done() <-chan struct{}
	Closed() bool
}

func FindTCPConn(c RawConn) *net.TCPConn {
	tcpConn, ok := c.NetConn().(*net.TCPConn)
	if ok {
		return tcpConn
	}

	rawConn, ok := c.NetConn().(RawConn)
	if !ok {
		return nil
	}

	return FindTCPConn(rawConn)
}

type doneConn struct {
	c         net.Conn
	closeChan chan struct{}

	closed      bool
	closedMutex sync.RWMutex
}

func (d *doneConn) Read(b []byte) (n int, err error) {
	return d.c.Read(b)
}

func (d *doneConn) Write(b []byte) (n int, err error) {
	return d.c.Write(b)
}

func (d *doneConn) Close() error {
	d.closedMutex.Lock()
	defer d.closedMutex.Unlock()
	if !d.closed {
		d.closed = true
		err := d.c.Close()
		close(d.closeChan)
		return err
	}
	return net.ErrClosed
}

func (d *doneConn) Closed() bool {
	d.closedMutex.RLock()
	defer d.closedMutex.RUnlock()
	return d.closed
}

func (d *doneConn) LocalAddr() net.Addr {
	return d.c.LocalAddr()
}

func (d *doneConn) RemoteAddr() net.Addr {
	return d.c.RemoteAddr()
}

func (d *doneConn) SetDeadline(t time.Time) error {
	return d.c.SetDeadline(t)
}

func (d *doneConn) SetReadDeadline(t time.Time) error {
	return d.c.SetReadDeadline(t)
}

func (d *doneConn) SetWriteDeadline(t time.Time) error {
	return d.c.SetWriteDeadline(t)
}

func (d *doneConn) Done() <-chan struct{} {
	return d.closeChan
}

func (d *doneConn) NetConn() net.Conn {
	return d.c
}

func NewDoneConn(c net.Conn) DoneConn {
	return &doneConn{
		c:         c,
		closeChan: make(chan struct{}),
	}
}
