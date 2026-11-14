package http

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
)

// serveLifecycle records the serve loop that Server.Start launches, so the
// server can report how it ended. The zero value is ready to use.
type serveLifecycle struct {
	mu      sync.Mutex
	started bool
	done    chan struct{}
	err     error
	failed  chan error
}

// failedChannel returns the failure channel, creating it on first use. It is
// called with l.mu held.
func (l *serveLifecycle) failedChannel() chan error {
	if l.failed == nil {
		l.failed = make(chan error, 1)
	}

	return l.failed
}

// begin marks the serve loop as started. It reports false if there already is
// one.
func (l *serveLifecycle) begin() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.started {
		return false
	}
	l.started = true
	l.done = make(chan struct{})
	l.failedChannel()

	return true
}

// abort undoes begin, for a Start whose bind failed.
func (l *serveLifecycle) abort() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.started = false
	l.done = nil
}

// end records how the serve loop ended, and reports an unexpected end on the
// failure channel.
func (l *serveLifecycle) end(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.err = err
	close(l.done)
	if isUnexpectedServeError(err) {
		l.failedChannel() <- err
	}
}

// wait blocks until the serve loop, if one was started, has returned or ctx is
// done. It returns the error that ended the loop unless it was an orderly one.
func (l *serveLifecycle) wait(ctx context.Context) error {
	l.mu.Lock()
	done := l.done
	l.mu.Unlock()
	if done == nil {
		return nil
	}

	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	return l.unexpected()
}

// unexpected returns the error that ended the serve loop unless it was an
// orderly end. It is only meaningful once the loop has ended.
func (l *serveLifecycle) unexpected() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if isUnexpectedServeError(l.err) {
		return l.err
	}

	return nil
}

// isUnexpectedServeError reports whether err ended a serve loop other than by
// Shutdown or by closing the listener.
func isUnexpectedServeError(err error) bool {
	return err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed)
}

// Start binds the address in Config.ListenAddress and serves on it in a
// goroutine, using the TLS configuration in Config.TLSConfig, if any. It
// returns once the listener is bound, so a bind failure (port in use, bad
// address) is returned to the caller instead of being logged from a goroutine
// after the caller has moved on.
//
// After Start, Failed reports a serve loop that ends unexpectedly, Live reports
// whether it is still running, and Shutdown waits for it to return. A Server
// can be started once.
func (s *Server) Start(ctx context.Context) error {
	if !s.lifecycle.begin() {
		return errors.New("server already started")
	}

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", s.Config.ListenAddress)
	if err != nil {
		s.lifecycle.abort()
		s.Logger.Error().Err(err).Str("address", s.Config.ListenAddress).Msg("failed to listen on address")

		return fmt.Errorf("failed to listen on %s: %w", s.Config.ListenAddress, err)
	}

	go func() {
		s.lifecycle.end(s.Serve(ln))
	}()

	return nil
}

// Failed returns a channel that receives, at most once, the error that ended
// the serve loop started by Start other than by Shutdown or by closing the
// listener. It never receives for a server that was not started with Start.
func (s *Server) Failed() <-chan error {
	s.lifecycle.mu.Lock()
	defer s.lifecycle.mu.Unlock()

	return s.lifecycle.failedChannel()
}

// Live returns nil while the serve loop started by Start is running, and for a
// server that has not been started or that Shutdown stopped. It returns the
// error that ended the serve loop if that was unexpected.
func (s *Server) Live(context.Context) error {
	s.lifecycle.mu.Lock()
	done := s.lifecycle.done
	s.lifecycle.mu.Unlock()
	if done == nil {
		return nil
	}

	select {
	case <-done:
		return s.lifecycle.unexpected()
	default:
		return nil
	}
}
