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

func structuredLogger(r *http.Request, status, size int, duration time.Duration) {
	hlog.FromRequest(r).Info().
		Str("method", r.Method).
		Stringer("url", r.URL).
		Int("status", status).
		Int("size", size).
		Dur("duration", duration).
		Str("user_agent", r.UserAgent()).
		Str("referer", r.Referer()).
		Str("ip", r.RemoteAddr).
		Str("proto", r.Proto).
		Str("host", r.Host).
		Msg("accessLog")
}

func ZerologStructuredLogHandler(logger zerolog.Logger) HandlerWrapper {
	logReq := hlog.NewHandler(logger)
	return func(next http.Handler) http.Handler {
		return logReq(hlog.AccessHandler(structuredLogger)(next))
	}
}
