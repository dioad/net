package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLimitBodySize_UnderLimit(t *testing.T) {
	l := NewBodySizeLimiter(WithMaxBodyBytes(1024))
	handler := l.Wrap(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("hello"))
	rec := httptest.NewRecorder()

	handler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "hello", rec.Body.String())
}

func TestLimitBodySize_OverLimit_ContentLength(t *testing.T) {
	l := NewBodySizeLimiter(WithMaxBodyBytes(10))

	handler := l.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	body := strings.NewReader("this is way more than 10 bytes")
	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.ContentLength = int64(body.Len())
	rec := httptest.NewRecorder()

	handler(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

func TestLimitBodySize_OverLimit_MaxBytesReader(t *testing.T) {
	l := NewBodySizeLimiter(WithMaxBodyBytes(10))
	handler := l.Wrap(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Don't set Content-Length to test MaxBytesReader path
	body := strings.NewReader("this is way more than 10 bytes")
	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.ContentLength = -1 // Unknown length
	rec := httptest.NewRecorder()

	handler(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

func TestLimitBodySize_DefaultLimit(t *testing.T) {
	l := NewBodySizeLimiter() // Use default limits
	handler := l.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("test"))
	rec := httptest.NewRecorder()

	handler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestLimitBodySize_MaxLimit(t *testing.T) {
	// Request a limit higher than MaxBodyBytes, should be capped
	l := NewBodySizeLimiter(WithMaxBodyBytes(100 * 1024 * 1024))

	handler := l.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("test"))
	rec := httptest.NewRecorder()

	handler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
