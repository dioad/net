package http

import (
	"io"
	"strings"

	"net/http"
	"time"

	"github.com/gorilla/handlers"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
)

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
func StandardLogger(r *http.Request, status, size int, duration time.Duration) *zerolog.Logger {
	ctx := hlog.FromRequest(r).With().
		Str("method", r.Method).
		Stringer("url", r.URL).
		Int("status", status).
		Int("size", size).
		Dur("duration", duration).
		Str("user_agent", r.UserAgent()).
		Str("referer", r.Referer()).
		Str("resolved_client_ip", GetClientIP(r)).
		Str("remote_addr", r.RemoteAddr).
		Str("proto", r.Proto).
		Str("host", r.Host)

	for _, h := range []string{"X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "Forwarded", "Via", "X-Real-IP"} {
		if v := r.Header.Get(h); v != "" {
			ctx = ctx.Str(headerToSnakeCase(h), v)
		}
	}

	logger := ctx.Logger()
	return &logger
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

		return logReq(hlog.AccessHandler(structuredLogger)(next))
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
	expiresAt time.Time
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
	err := z.SetLogLevel(level)
	if err != nil {
		return time.Time{}, nil // Return zero time if there was an error
	}

	z.expiresAt = time.Now().Add(duration)
	go func() {
		time.AfterFunc(duration, z.ResetLogLevel)
	}()

	return z.expiresAt, nil
}

// ExpiresAt returns the time when the current log level will expire.
func (z *zerologLevelSetter) ExpiresAt() time.Time {
	return z.expiresAt
}

// ResetLogLevel resets the log level to the original level.
func (z *zerologLevelSetter) ResetLogLevel() {
	zerolog.SetGlobalLevel(z.originalLevel)
	z.expiresAt = time.Time{}
}
