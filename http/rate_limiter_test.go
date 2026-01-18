package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_Middleware(t *testing.T) {
	logger := zerolog.New(zerolog.NewConsoleWriter())
	// 1 token per second, burst of 1
	rl := NewRateLimiter(1, 1, logger)

	handler := rl.Middleware("user1")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request - allowed
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// Second request - rate limited
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Equal(t, "60", rr.Header().Get("Retry-After"))
}

func TestRateLimiter_MiddlewareFromContext(t *testing.T) {
	logger := zerolog.New(zerolog.NewConsoleWriter())
	rl := NewRateLimiter(1, 1, logger)

	type contextKey string
	key := contextKey("principal")

	handler := rl.MiddlewareFromContext(key)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request with principal in context
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), key, "user1"))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// Request without principal in context
	req = httptest.NewRequest("GET", "/", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	// Request with empty principal
	req = httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), key, ""))
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	// Request with principal that gets rate limited
	rl2 := NewRateLimiter(1, 1, logger)
	handler2 := rl2.MiddlewareFromContext(key)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req = httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), key, "user1"))
	rr = httptest.NewRecorder()
	handler2.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	handler2.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}
