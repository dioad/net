package http

import (
	"context"
	"io"
	"strings"

	"net/http"
	"sync"
	"time"

	"github.com/gorilla/handlers"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
)

// requestFieldsInjectedKey is the context key for a per-request *bool shared
// between Server.AddResource, which injects method, url, host, remote_addr,
// and user_agent into the context logger mid-request so service-layer code
// can log them without recomputing them, and StandardLogger, which emits the
// final "accessLog" event from that same context logger. AddResource sets
// the flag to true after injecting those fields; StandardLogger checks it to
// avoid re-adding them, since zerolog fields are appended rather than
// replaced and a second .Str() call for the same key would duplicate it in
// the emitted JSON.
//
// The flag is only present when the handler chain was built by
// ZerologStructuredLogHandlerWithFormatter, so callers of StandardLogger
// outside that chain (e.g. a custom formatter wired directly onto another
// server type) see no flag and get the fields as before.
type requestFieldsInjectedKey struct{}

// HandlerWrapper is a function type that wraps an HTTP handler.
type HandlerWrapper func(next http.Handler) http.Handler

// DefaultCombinedLogHandler returns a HandlerWrapper that logs HTTP requests using the combined log format.
// It wraps the handler with ProxyHeaders middleware so that X-Forwarded-For and X-Real-IP
// headers are reflected in the logged client IP.
func DefaultCombinedLogHandler(logWriter io.Writer) HandlerWrapper {
	return func(next http.Handler) http.Handler {
		return handlers.CombinedLoggingHandler(logWriter, handlers.ProxyHeaders(next))
	}
}

// StructuredLoggerFormatter is a function type that formats HTTP request logs in a structured format.
type StructuredLoggerFormatter func(r *http.Request, status, size int, duration time.Duration) *zerolog.Logger

func headerToSnakeCase(s string) string {
	lower := strings.ToLower(s)

	return strings.ReplaceAll(lower, "-", "_")
}

// StandardLogger creates a zerolog.Logger with standard fields for HTTP access logging.
// The "ip" field is resolved from X-Forwarded-For or X-Real-IP headers when present,
// falling back to RemoteAddr. Raw proxy headers and RemoteAddr are also included when set.
//
// "resolved_client_ip" is only as trustworthy as GetClientIP's input: absent
// a trusted, header-stripping proxy in front of this process, any client
// can set it to an arbitrary value. It is logged alongside the raw
// "remote_addr" and proxy headers precisely so a reader can cross-check it
// rather than treat it as verified; do not use it alone as an audit trail
// of a request's true origin.
func StandardLogger(r *http.Request, status, size int, duration time.Duration) *zerolog.Logger {
	ctx := hlog.FromRequest(r).With().
		Int("status", status).
		Int("size", size).
		Dur("duration", duration).
		Str("referer", r.Referer()).
		Str("resolved_client_ip", GetClientIP(r)).
		Str("proto", r.Proto)

	if injected, ok := r.Context().Value(requestFieldsInjectedKey{}).(*bool); !ok || !*injected {
		ctx = ctx.Str("method", r.Method).
			Stringer("url", r.URL).
			Str("user_agent", r.UserAgent()).
			Str("remote_addr", r.RemoteAddr).
			Str("host", r.Host)
	}

	for _, h := range []string{"X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "Forwarded", "Via", "X-Real-IP"} {
		if v := r.Header.Get(h); v != "" {
			ctx = ctx.Str(headerToSnakeCase(h), v)
		}
	}

	return new(ctx.Logger())
}

// func CoreDNSStandardLogger

// func structuredLogger(r *http.Request, status, size int, duration time.Duration) {
//	StandardLogger(r, status, size, duration).Info().Msg("accessLog")
// }

// ZerologStructuredLogHandler returns a HandlerWrapper that uses zerolog for structured logging.
func ZerologStructuredLogHandler(logger zerolog.Logger) HandlerWrapper {
	return ZerologStructuredLogHandlerWithFormatter(logger, StandardLogger)
}

// ZerologStructuredLogHandlerWithFormatter returns a HandlerWrapper that uses a custom formatter for structured logging.
func ZerologStructuredLogHandlerWithFormatter(logger zerolog.Logger, formatter StructuredLoggerFormatter) HandlerWrapper {
	logReq := hlog.NewHandler(logger)

	return func(next http.Handler) http.Handler {
		structuredLogger := func(r *http.Request, status, size int, duration time.Duration) {
			formatter(r, status, size, duration).Info().Msg("accessLog")
		}

		handler := logReq(hlog.AccessHandler(structuredLogger)(next))

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			injected := false
			ctx := context.WithValue(r.Context(), requestFieldsInjectedKey{}, &injected)
			handler.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// LogLevelSetter defines an interface for setting and managing  log levels.
type LogLevelSetter interface {
	// SetLogLevel sets the global log level for zerolog.
	SetLogLevel(level string) error
	// SetLogLevelWithDuration sets the global log level for zerolog and returns the time when it expires.
	SetLogLevelWithDuration(level string, duration time.Duration) (time.Time, error)
	// OriginalLogLevel returns the original log level before any changes.
	OriginalLogLevel() string
	// CurrentLogLevel returns the current log level.
	CurrentLogLevel() string
	// ResetLogLevel resets the log level to the original level.
	ResetLogLevel()
	// ExpiresAt returns the time when the current log level will expire.
	ExpiresAt() time.Time
}

// NewZeroLogLevelSetter creates a new LogLevelSetter for zerolog.
func NewZeroLogLevelSetter() LogLevelSetter {
	return &zerologLevelSetter{
		originalLevel: zerolog.GlobalLevel(),
	}
}

// zerologLevelSetter implements LogLevelSetter for zerolog.
type zerologLevelSetter struct {
	originalLevel zerolog.Level

	// save current state
	mu        sync.Mutex
	expiresAt time.Time
	timer     *time.Timer
}

// OriginalLogLevel returns the original log level of zerolog.
func (z *zerologLevelSetter) OriginalLogLevel() string {
	return z.originalLevel.String()
}

// CurrentLogLevel returns the current log level of zerolog.
func (z *zerologLevelSetter) CurrentLogLevel() string {
	return zerolog.GlobalLevel().String()
}

// SetLogLevel sets the global log level for zerolog.
func (z *zerologLevelSetter) SetLogLevel(level string) error {
	l, err := zerolog.ParseLevel(level)
	if err != nil {
		return err
	}

	zerolog.SetGlobalLevel(l)

	return nil
}

// SetLogLevelWithDuration sets the log level and returns the time when it expires.
func (z *zerologLevelSetter) SetLogLevelWithDuration(level string, duration time.Duration) (time.Time, error) {
	// Set the level first and propagate any error back to the caller.
	err := z.SetLogLevel(level)
	if err != nil {
		return time.Time{}, err
	}

	z.mu.Lock()
	defer z.mu.Unlock()

	// Stop previous timer if any.
	if z.timer != nil {
		z.timer.Stop()
		z.timer = nil
	}

	z.expiresAt = time.Now().Add(duration)
	// Schedule reset and save timer for potential cancellation.
	z.timer = time.AfterFunc(duration, func() {
		z.ResetLogLevel()
	})

	return z.expiresAt, nil
}

// ExpiresAt returns the time when the current log level will expire.
func (z *zerologLevelSetter) ExpiresAt() time.Time {
	z.mu.Lock()
	defer z.mu.Unlock()

	return z.expiresAt
}

// ResetLogLevel resets the log level to the original level.
func (z *zerologLevelSetter) ResetLogLevel() {
	z.mu.Lock()
	// Stop timer if present.
	if z.timer != nil {
		z.timer.Stop()
		z.timer = nil
	}
	z.expiresAt = time.Time{}
	z.mu.Unlock()

	zerolog.SetGlobalLevel(z.originalLevel)
}
