package http

import (
	"io"

	"net/http"
	"time"

	"github.com/gorilla/handlers"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
)

type HandlerWrapper func(next http.Handler) http.Handler

// DefaultCombinedLogHandler returns a HandlerWrapper that logs HTTP requests using the combined log format.
func DefaultCombinedLogHandler(logWriter io.Writer) HandlerWrapper {
	return func(next http.Handler) http.Handler {
		return handlers.CombinedLoggingHandler(logWriter, next)
	}
}

type StructuredLoggerFormatter func(r *http.Request, status, size int, duration time.Duration) *zerolog.Logger

// StandardLogger creates a zerolog.Logger with standard fields for HTTP access logging.
func StandardLogger(r *http.Request, status, size int, duration time.Duration) *zerolog.Logger {
	logger := hlog.FromRequest(r).With().
		Str("method", r.Method).
		Stringer("url", r.URL).
		Int("status", status).
		Int("size", size).
		Dur("duration", duration).
		Str("user_agent", r.UserAgent()).
		Str("referer", r.Referer()).
		Str("ip", r.RemoteAddr).
		Str("proto", r.Proto).
		Str("host", r.Host).Logger()

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
