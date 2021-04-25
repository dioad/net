package metrics

import (
	"net"
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
	BytesRead() int
	BytesWritten() int
	ResetMetrics()
	Duration() time.Duration
	StartTime() time.Time
	EndTime() time.Time
}

type connMetrics struct {
	bytesRead    int
	bytesWritten int

	startTime time.Time
	endTime   time.Time
}

type Conn struct {
	conn    net.Conn
	metrics *connMetrics
}

func (s *Conn) StartTime() time.Time {
	return s.metrics.startTime
}

func (s *Conn) SetStartTime(t time.Time) {
	s.metrics.startTime = t
}

func (s *Conn) EndTime() time.Time {
	return s.metrics.endTime
}

func (s *Conn) SetEndTime(t time.Time) {
	s.metrics.endTime = t
}

func (s *Conn) Duration() time.Duration {
	if s.metrics.endTime.IsZero() {
		return time.Since(s.metrics.startTime)
	}

	return s.metrics.endTime.Sub(s.metrics.startTime)
}

func (s *Conn) BytesRead() int {
	return s.metrics.bytesRead
}

func (s *Conn) BytesWritten() int {
	return s.metrics.bytesWritten
}

func (s *Conn) ResetMetrics() {
	s.metrics = &connMetrics{}
}

func (s Conn) Read(b []byte) (int, error) {
	n, err := s.conn.Read(b)
	s.metrics.bytesRead += n
	return n, err
}

func (s Conn) Write(b []byte) (int, error) {
	n, err := s.conn.Write(b)
	s.metrics.bytesWritten += n
	return n, err
}

func (s Conn) Close() error {
	s.SetEndTime(time.Now())
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

func NewConn(c net.Conn) net.Conn {
	return NewConnWithStartTime(c, time.Now())
}

func NewConnWithStartTime(c net.Conn, startTime time.Time) net.Conn {
	return &Conn{
		conn: c,
		metrics: &connMetrics{
			startTime: startTime,
		},
	}
}

func NewConnWithLogger(c net.Conn, logger zerolog.Logger) net.Conn {
	metricsConn := NewConn(c)
	closerConn := net2.NewConnWithCloser(metricsConn, func(c net.Conn) {
		c.Close()
		if metricsConn, ok := c.(*Conn); ok {
			logger.Info().Int("bytesRead", metricsConn.BytesRead()).
				Int("bytesWritten", metricsConn.BytesWritten()).
				Time("startTime", metricsConn.StartTime()).
				Time("endTime", metricsConn.EndTime()).
				Dur("duration", metricsConn.Duration()).
				Msg("connectionStats")
		}
	})
	return closerConn
}
