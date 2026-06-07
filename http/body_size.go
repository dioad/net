package http

import (
	"net/http"

	"github.com/rs/zerolog"
)

const (
	// DefaultMaxBodyBytes is the default maximum request body size (1MB).
	DefaultMaxBodyBytes = 1 * 1024 * 1024
)

// BodySizeLimiter is a middleware that limits the size of incoming request bodies.
type BodySizeLimiter struct {
	MaxBodyBytes int64
}

// BodySizeLimiterOpt defines a functional option for configuring the BodySizeLimiter.
type BodySizeLimiterOpt func(*BodySizeLimiter)

// WithBodySizeLimiterLogger is a no-op kept for API compatibility.
//
// Deprecated: BodySizeLimiter now uses the request-scoped zerolog context logger
// (zerolog.Ctx(r.Context())) directly inside Wrap, so no external logger is needed.
func WithBodySizeLimiterLogger(_ zerolog.Logger) BodySizeLimiterOpt {
	return func(_ *BodySizeLimiter) {}
}

// WithMaxBodyBytes sets the maximum allowed body size for requests. If not set, DefaultMaxBodyBytes is used.
func WithMaxBodyBytes(maxBytesSize int64) BodySizeLimiterOpt {
	return func(l *BodySizeLimiter) {
		l.MaxBodyBytes = maxBytesSize
	}
}

// NewBodySizeLimiter creates a new BodySizeLimiter with the provided options.
func NewBodySizeLimiter(opts ...BodySizeLimiterOpt) *BodySizeLimiter {
	l := &BodySizeLimiter{
		MaxBodyBytes: DefaultMaxBodyBytes,
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// Wrap wraps an http.HandlerFunc to limit the request body size.
// Rejection events are logged using the request-scoped zerolog context logger so that
// request_id, principal, and other context fields are automatically included.
func (l *BodySizeLimiter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check Content-Length header first for early rejection
		if r.ContentLength > l.MaxBodyBytes {
			zerolog.Ctx(r.Context()).Warn().
				Int64("content_length", r.ContentLength).
				Int64("max_bytes", l.MaxBodyBytes).
				Str("path", r.URL.Path).
				Msg("Request body too large (Content-Length)")

			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}

		// Wrap the body with a size limiter
		r.Body = http.MaxBytesReader(w, r.Body, l.MaxBodyBytes)

		next.ServeHTTP(w, r)
	})
}
