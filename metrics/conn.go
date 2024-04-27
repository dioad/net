package metrics

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	net2 "github.com/dioad/net"
	"github.com/rs/zerolog"
)

// ReadWriteCounter is an interface that exposes the number of bytes read and
// written.
type ReadWriteCounter interface {
	BytesRead()
	BytesWritten()
}

// ReadCounter is an interface that exposes the number of bytes read.
type ReadCounter interface {
	BytesRead() int
}

// WrittenCounter is an interface that exposes the number of bytes written.
type WrittenCounter interface {
	BytesWritten() int
}

// ConnMetrics is an interface that exposes metrics for a connection.
type ConnMetrics interface {
	BytesRead() uint64
	BytesWritten() uint64
	ResetMetrics()
	Duration() time.Duration
	StartTime() time.Time
	EndTime() time.Time
}

// NewConnWithLogger returns a new net.Conn that wraps the given net.Conn and
// logs connection stats when the connection is closed.
type connMetrics struct {
	bytesRead    uint64
	bytesWritten uint64

	startTime time.Time
	endTime   time.Time

	timeMutex sync.Mutex
}

// BytesRead returns the number of bytes read.
func (m *connMetrics) BytesRead() uint64 {
	return atomic.LoadUint64(&m.bytesRead)
}

// BytesWritten returns the number of bytes written.
func (m *connMetrics) BytesWritten() uint64 {
	return atomic.LoadUint64(&m.bytesWritten)
}

// IncBytesRead increments the number of bytes read.
func (m *connMetrics) IncBytesRead(n int) {
	atomic.AddUint64(&m.bytesRead, uint64(n))
	m.updateTime()
}

// updateTime updates the start and end time of the connection.
func (m *connMetrics) updateTime() {
	m.timeMutex.Lock()
	defer m.timeMutex.Unlock()
	now := time.Now()
	// TODO: replace this with a sync.Once
	if m.startTime.IsZero() {
		m.startTime = now
	}
	m.endTime = now
}

// IncBytesWritten increments the number of bytes written.
func (m *connMetrics) IncBytesWritten(n int) {
	atomic.AddUint64(&m.bytesWritten, uint64(n))
	m.updateTime()
}

// StartTime returns the time when the connection was opened.
func (m *connMetrics) StartTime() time.Time {
	m.timeMutex.Lock()
	defer m.timeMutex.Unlock()

	return m.startTime
}

// EndTime returns the time when the connection was closed.
func (m *connMetrics) EndTime() time.Time {
	m.timeMutex.Lock()
	defer m.timeMutex.Unlock()

	return m.endTime
}

// Duration returns the duration of the connection.
func (m *connMetrics) Duration() time.Duration {
	m.timeMutex.Lock()
	defer m.timeMutex.Unlock()

	return m.endTime.Sub(m.startTime)
}

// Conn is a net.Conn that records metrics.
type Conn struct {
	conn    net.Conn
	metrics *connMetrics
}

// NetConn returns the underlying net.Conn.
func (s *Conn) NetConn() net.Conn {
	return s.conn
}

// StartTime returns the time when the connection was opened.
func (s *Conn) StartTime() time.Time {
	return s.metrics.StartTime()
}

// EndTime returns the time when the connection was closed.
func (s *Conn) EndTime() time.Time {
	return s.metrics.EndTime()
}

// Duration returns the duration of the connection.
func (s *Conn) Duration() time.Duration {
	return s.metrics.Duration()
}

// BytesRead returns the number of bytes read.
func (s *Conn) BytesRead() uint64 {
	return s.metrics.BytesRead()
}

// BytesWritten returns the number of bytes written.
func (s *Conn) BytesWritten() uint64 {
	return s.metrics.BytesWritten()
}

// ResetMetrics resets the metrics.
func (s *Conn) ResetMetrics() {
	s.metrics = &connMetrics{}
}

// Read reads data from the connection.
func (s *Conn) Read(b []byte) (int, error) {
	n, err := s.conn.Read(b)
	s.metrics.IncBytesRead(n)
	return n, err
}

// Write writes data to the connection.
func (s *Conn) Write(b []byte) (int, error) {
	n, err := s.conn.Write(b)
	s.metrics.IncBytesWritten(n)
	return n, err
}

// Close closes the connection.
func (s *Conn) Close() error {
	return s.conn.Close()
}

// LocalAddr returns the local network address.
func (s *Conn) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (s *Conn) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

// SetDeadline sets the read and write deadlines associated with the
func (s *Conn) SetDeadline(t time.Time) error {
	return s.conn.SetDeadline(t)
}

// SetReadDeadline sets the deadline for future Read calls.
func (s *Conn) SetReadDeadline(t time.Time) error {
	return s.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the deadline for future Write calls.
func (s *Conn) SetWriteDeadline(t time.Time) error {
	return s.conn.SetWriteDeadline(t)
}

// SetKeepAlive sets whether the operating system should send keepalive
func (s *Conn) SetKeepAlive(keepalive bool) error {
	if c, ok := s.conn.(*net.TCPConn); ok {
		return c.SetKeepAlive(keepalive)
	}
	return nil
}

// SetKeepAlivePeriod sets period between keep alives.
func (s *Conn) SetKeepAlivePeriod(d time.Duration) error {
	if c, ok := s.conn.(*net.TCPConn); ok {
		return c.SetKeepAlivePeriod(d)
	}
	return nil
}

// NewConn returns a new net.Conn that wraps the given net.Conn and starts
// recording metrics from the current time.
func NewConn(c net.Conn) net.Conn {
	return NewConnWithStartTime(c, time.Now())
}

// NewConnWithStartTime returns a new net.Conn that wraps the given net.Conn and
// starts recording metrics from the given startTime.
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

// NewConnWithCloser returns a new net.Conn that wraps the given net.Conn and
func NewConnWithCloser(c net.Conn, closer func(net.Conn)) net.Conn {
	conn := NewConn(c)

	return net2.NewConnWithCloser(conn, closer)
}

// NewConnWithLogger returns a new net.Conn that wraps the given net.Conn and
// logs connection stats when the connection is closed.
func NewConnWithLogger(c net.Conn, logger zerolog.Logger) net.Conn {
	return NewConnWithCloser(c, func(c net.Conn) {
		err := c.Close()
		if err != nil {
			logger.Error().
				Err(err).
				Msg("connectionCloseError")
		}
		if metricsConn, ok := c.(*Conn); ok {
			logger.Info().
				Str("localAddr", c.LocalAddr().String()).
				Str("remoteAddr", c.RemoteAddr().String()).
				Uint64("bytesRead", metricsConn.BytesRead()).
				Uint64("bytesWritten", metricsConn.BytesWritten()).
				Time("startTime", metricsConn.StartTime()).
				Time("endTime", metricsConn.EndTime()).
				Dur("duration", metricsConn.Duration()).
				Msg("connectionStats")
		}
		logger.Trace().
			Msg("connectionClosed")
	})
}
