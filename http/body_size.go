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
	Logger       zerolog.Logger
}

// BodySizeLimiterOpt defines a functional option for configuring the BodySizeLimiter.
type BodySizeLimiterOpt func(*BodySizeLimiter)

// WithBodySizeLimiterLogger sets a custom logger for the BodySizeLimiter.
func WithBodySizeLimiterLogger(logger zerolog.Logger) BodySizeLimiterOpt {
	return func(l *BodySizeLimiter) {
		l.Logger = logger
	}
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
func (l *BodySizeLimiter) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check Content-Length header first for early rejection
		if r.ContentLength > l.MaxBodyBytes {
			l.Logger.Warn().
				Int64("content_length", r.ContentLength).
				Int64("max_bytes", l.MaxBodyBytes).
				Str("path", r.URL.Path).
				Msg("Request body too large (Content-Length)")

			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}

		// Wrap the body with a size limiter
		r.Body = http.MaxBytesReader(w, r.Body, l.MaxBodyBytes)

		next(w, r)
	}
}
