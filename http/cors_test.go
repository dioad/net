package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestCORSConfig_AllowsOnlyWhatItLists(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		cfg    CORSConfig
		origin string
		want   bool
	}{
		{"empty config is closed", CORSConfig{}, "https://app.example.com", false},
		{"listed origin", CORSConfig{AllowedOrigins: []string{"https://app.example.com"}}, "https://app.example.com", true},
		{"unlisted origin", CORSConfig{AllowedOrigins: []string{"https://app.example.com"}}, "https://evil.example.com", false},
		{"wildcard", CORSConfig{AllowedOrigins: []string{"*"}}, "https://anything.example.com", true},
		{"localhost not allowed by default", CORSConfig{}, "http://localhost:3000", false},
		{"localhost when allowed", CORSConfig{AllowLocalhost: true}, "http://localhost:3000", true},
		{"allowing localhost keeps other origins out", CORSConfig{AllowLocalhost: true}, "https://evil.example.com", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			req.Header.Set("Origin", tc.origin)
			tc.cfg.Middleware(zerolog.Nop(), false)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rec, req)

			got := rec.Header().Get("Access-Control-Allow-Origin") == tc.origin
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCORSConfig_OptionsApplyDefaults(t *testing.T) {
	t.Parallel()

	got := CORSConfig{}.Options(zerolog.Nop(), false)

	assert.Equal(t, []string{"Content-Type", "Authorization"}, got.AllowedHeaders)
	assert.Equal(t, 24*60*60, got.MaxAge)
	assert.False(t, got.AllowCredentials, "credentials are opt-in")
}

func TestCORSConfig_OptionsKeepWhatIsSet(t *testing.T) {
	t.Parallel()

	got := CORSConfig{
		AllowedHeaders:      []string{"X-Custom"},
		AllowedMethods:      []string{"GET", "DELETE"},
		ExposedHeaders:      []string{"X-Request-Id"},
		AllowCredentials:    true,
		AllowPrivateNetwork: true,
		MaxAgeSeconds:       60,
	}.Options(zerolog.Nop(), true)

	assert.Equal(t, []string{"X-Custom"}, got.AllowedHeaders)
	assert.Equal(t, []string{"GET", "DELETE"}, got.AllowedMethods)
	assert.Equal(t, []string{"X-Request-Id"}, got.ExposedHeaders)
	assert.True(t, got.AllowCredentials)
	assert.True(t, got.AllowPrivateNetwork)
	assert.Equal(t, 60, got.MaxAge)
	assert.True(t, got.Debug)
}
