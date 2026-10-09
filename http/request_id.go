package http

import (
	"net/http"

	"uuid"

	"github.com/rs/zerolog"
)

const requestIDHeader = "X-Request-ID"

// RequestIDMiddleware always generates a fresh UUID v4 for every request,
// sets it on the response as X-Request-ID, and enriches the context logger
// with request_id via UpdateContext so the field is visible to all code that
// reads zerolog.Ctx(r.Context()) — including deferred access-log callbacks
// that captured r before this middleware ran.
//
// The request's own X-Request-ID header is intentionally ignored so that
// clients cannot influence the ID that appears in server logs.
//
// Handlers retrieve the per-request logger with:
//
//	logger := zerolog.Ctx(r.Context())
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New()

		w.Header().Set(requestIDHeader, requestID.String())
		zerolog.Ctx(r.Context()).UpdateContext(func(c zerolog.Context) zerolog.Context {
			return c.Stringer("request_id", requestID)
		})
		next.ServeHTTP(w, r)
	})
}
