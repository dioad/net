package ratelimit

import (
	"net"

	"github.com/rs/zerolog"
)

// Listener is a network listener that enforces rate limiting on all incoming connections.
type Listener struct {
	net.Listener
	RateLimiter *RateLimiter
	Logger      zerolog.Logger
}

// NewListener creates a new rate-limiting listener.
func NewListener(l net.Listener, rl *RateLimiter, logger zerolog.Logger) *Listener {
	return &Listener{
		Listener:    l,
		RateLimiter: rl,
		Logger:      logger,
	}
}

// Accept waits for and returns the next connection to the listener.
// It checks each connection's source IP against the RateLimiter and closes it if the limit is exceeded.
func (l *Listener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}

		principal := l.getPrincipal(conn)
		if !l.RateLimiter.Allow(principal) {
			l.Logger.Warn().
				Str("remoteAddr", conn.RemoteAddr().String()).
				Str("principal", principal).
				Msg("rate limit exceeded, rejecting connection")
			conn.Close()
			continue
		}

		return conn, nil
	}
}

func (l *Listener) getPrincipal(conn net.Conn) string {
	remoteAddr := conn.RemoteAddr().String()

	// Try to extract IP if it's in host:port format
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}

	// Fallback to full address string (e.g. for Unix sockets or if SplitHostPort fails)
	return remoteAddr
}
