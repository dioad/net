package http

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

const requestIDHeader = "X-Request-ID"

// RequestIDMiddleware always generates a fresh UUID v4 for every request,
// sets it on the response as X-Request-ID, and injects a per-request logger
// carrying request_id into the request context.
//
// The request's own X-Request-ID header is intentionally ignored so that
// clients cannot influence the ID that appears in server logs.
//
// Handlers retrieve the per-request logger with:
//
//	logger := zerolog.Ctx(r.Context())
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.NewString()
		w.Header().Set(requestIDHeader, requestID)
		logger := zerolog.Ctx(r.Context()).With().Str("request_id", requestID).Logger()
		next.ServeHTTP(w, r.WithContext(logger.WithContext(r.Context())))
	})
}
