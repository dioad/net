package metrics

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	net2 "github.com/dioad/net"
	"github.com/rs/zerolog"
)

type ReadWriteCounter interface {
	BytesRead()
	BytesWritten()
}

type ReadCounter interface {
	BytesRead() int
}

type WrittenCounter interface {
	BytesWritten() int
}

type ConnMetrics interface {
	BytesRead() uint64
	BytesWritten() uint64
	ResetMetrics()
	Duration() time.Duration
	StartTime() time.Time
	EndTime() time.Time
}

type connMetrics struct {
	bytesRead    uint64
	bytesWritten uint64

	startTime time.Time
	endTime   time.Time

	timeMutex sync.Mutex
}

func (m *connMetrics) BytesRead() uint64 {
	return atomic.LoadUint64(&m.bytesRead)
}

func (m *connMetrics) BytesWritten() uint64 {
	return atomic.LoadUint64(&m.bytesWritten)
}

func (m *connMetrics) IncBytesRead(n int) {
	atomic.AddUint64(&m.bytesRead, uint64(n))
	m.updateTime()
}

func (m *connMetrics) updateTime() {
	m.timeMutex.Lock()
	defer m.timeMutex.Unlock()
	now := time.Now()
	if m.startTime.IsZero() {
		m.startTime = now
	}
	m.endTime = now
}

func (m *connMetrics) IncBytesWritten(n int) {
	atomic.AddUint64(&m.bytesWritten, uint64(n))
	m.updateTime()
}

func (m *connMetrics) StartTime() time.Time {
	m.timeMutex.Lock()
	defer m.timeMutex.Unlock()

	return m.startTime
}

func (m *connMetrics) EndTime() time.Time {
	m.timeMutex.Lock()
	defer m.timeMutex.Unlock()

	return m.endTime
}

func (m *connMetrics) Duration() time.Duration {
	m.timeMutex.Lock()
	defer m.timeMutex.Unlock()

	return m.endTime.Sub(m.startTime)
}

type Conn struct {
	conn    net.Conn
	metrics *connMetrics
}

func (s *Conn) StartTime() time.Time {
	return s.metrics.StartTime()
}

func (s *Conn) EndTime() time.Time {
	return s.metrics.EndTime()
}

func (s *Conn) Duration() time.Duration {
	return s.metrics.Duration()
}

func (s *Conn) BytesRead() uint64 {
	return s.metrics.BytesRead()
}

func (s *Conn) BytesWritten() uint64 {
	return s.metrics.BytesWritten()
}

func (s *Conn) ResetMetrics() {
	s.metrics = &connMetrics{}
}

func (s Conn) Read(b []byte) (int, error) {
	n, err := s.conn.Read(b)
	s.metrics.IncBytesRead(n)
	return n, err
}

func (s Conn) Write(b []byte) (int, error) {
	n, err := s.conn.Write(b)
	s.metrics.IncBytesWritten(n)
	return n, err
}

func (s Conn) Close() error {
	return s.conn.Close()
}

func (s Conn) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

func (s Conn) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

func (s Conn) SetDeadline(t time.Time) error {
	return s.conn.SetDeadline(t)
}

func (s Conn) SetReadDeadline(t time.Time) error {
	return s.conn.SetReadDeadline(t)
}

func (s Conn) SetWriteDeadline(t time.Time) error {
	return s.conn.SetWriteDeadline(t)
}

func (s Conn) SetKeepAlive(keepalive bool) error {
	if c, ok := s.conn.(*net.TCPConn); ok {
		return c.SetKeepAlive(keepalive)
	}
	return nil
}

func (s Conn) SetKeepAlivePeriod(d time.Duration) error {
	if c, ok := s.conn.(*net.TCPConn); ok {
		return c.SetKeepAlivePeriod(d)
	}
	return nil
}

func NewConn(c net.Conn) net.Conn {
	return NewConnWithStartTime(c, time.Now())
}

func NewConnWithStartTime(c net.Conn, startTime time.Time) net.Conn {
	conn := &Conn{
		conn: c,
		metrics: &connMetrics{
			startTime: startTime,
		},
	}

	conn.SetKeepAlive(true)
	conn.SetKeepAlivePeriod(60 * time.Minute)

	return conn
}

func NewConnWithLogger(c net.Conn, logger zerolog.Logger) net.Conn {
	metricsConn := NewConn(c)
	closerConn := net2.NewConnWithCloser(metricsConn, func(c net.Conn) {
		c.Close()
		if metricsConn, ok := c.(*Conn); ok {
			logger.Info().
				Uint64("bytesRead", metricsConn.BytesRead()).
				Uint64("bytesWritten", metricsConn.BytesWritten()).
				Time("startTime", metricsConn.StartTime()).
				Time("endTime", metricsConn.EndTime()).
				Dur("duration", metricsConn.Duration()).
				Msg("connectionStats")
		}
		logger.Info().
			Msg("connectionClosed")
	})
	return closerConn
}
