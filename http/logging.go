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

func DefaultCombinedLogHandler(logWriter io.Writer) HandlerWrapper {
	return func(next http.Handler) http.Handler {
		return handlers.CombinedLoggingHandler(logWriter, next)
	}
}

type StructuredLoggerFormatter func(r *http.Request, status, size int, duration time.Duration) *zerolog.Logger

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

func ZerologStructuredLogHandler(logger zerolog.Logger) HandlerWrapper {
	return ZerologStructuredLogHandlerWithFormatter(logger, StandardLogger)
}

func ZerologStructuredLogHandlerWithFormatter(logger zerolog.Logger, formatter StructuredLoggerFormatter) HandlerWrapper {
	logReq := hlog.NewHandler(logger)
	return func(next http.Handler) http.Handler {
		structuredLogger := func(r *http.Request, status, size int, duration time.Duration) {
			formatter(r, status, size, duration).Info().Msg("accessLog")
		}

		return logReq(hlog.AccessHandler(structuredLogger)(next))
	}
}
