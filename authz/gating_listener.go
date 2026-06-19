package authz

import "net"

// GatingListener wraps a net.Listener and applies a gate predicate to each
// accepted connection. Connections rejected by the gate are closed and the
// loop retries; only connections that pass the gate are returned.
//
// gate receives the accepted conn and returns true to allow it. A false return
// closes the connection and retries; gate errors are the gate's responsibility
// to handle before returning.
type GatingListener struct {
	net.Listener
	gate func(net.Conn) bool
}

// NewGatingListener creates a GatingListener that applies gate to every
// accepted connection. gate must be safe for concurrent use if Accept is
// called concurrently (in practice net.Listener.Accept is typically called
// from a single goroutine).
func NewGatingListener(l net.Listener, gate func(net.Conn) bool) *GatingListener {
	return &GatingListener{Listener: l, gate: gate}
}

// Accept waits for and returns the next connection that passes the gate.
func (g *GatingListener) Accept() (net.Conn, error) {
	for {
		c, err := g.Listener.Accept()
		if err != nil {
			return nil, err
		}
		if g.gate(c) {
			return c, nil
		}
		_ = c.Close()
	}
}
