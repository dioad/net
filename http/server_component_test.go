package http

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syncBuffer is a bytes.Buffer that is safe to write from the server's
// goroutines while the test reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

// events returns the JSON log events written so far.
func (b *syncBuffer) events(t *testing.T) []map[string]any {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()

	var events []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(b.buf.String()), "\n") {
		if line == "" {
			continue
		}
		var event map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &event))
		events = append(events, event)
	}

	return events
}

func TestWithComponent_LabelsTheServersOwnEvents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts func(zerolog.Logger) []ServerOption
	}{
		{"after WithLogger", func(l zerolog.Logger) []ServerOption {
			return []ServerOption{WithLogger(l), WithComponent("control-server")}
		}},
		{"before WithLogger", func(l zerolog.Logger) []ServerOption {
			return []ServerOption{WithComponent("control-server"), WithLogger(l)}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logs := &syncBuffer{}
			server := NewServer(Config{}, tt.opts(zerolog.New(logs))...)

			ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
			require.NoError(t, err)
			done := make(chan error, 1)
			go func() { done <- server.Serve(ln) }()
			require.NoError(t, server.Shutdown(t.Context()))
			require.ErrorIs(t, <-done, http.ErrServerClosed)

			messages := map[string]bool{}
			for _, event := range logs.events(t) {
				assert.Equal(t, "control-server", event["component"], "event %v", event["message"])
				messages[event["message"].(string)] = true //nolint:forcetypeassert // zerolog always writes a string message
			}
			assert.True(t, messages["starting server"])
			assert.True(t, messages["server stopped"])
		})
	}
}

func TestWithComponent_LabelsTheRequestLog(t *testing.T) {
	t.Parallel()
	logs := &syncBuffer{}
	server := NewServer(Config{}, WithLogger(zerolog.New(logs)), WithComponent("mgmt-server"))
	server.AddHandlerFunc("GET /ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	rec := httptest.NewRecorder()
	server.handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ping", nil))

	require.Equal(t, http.StatusNoContent, rec.Code)
	events := logs.events(t)
	require.NotEmpty(t, events, "the request is logged")
	for _, event := range events {
		assert.Equal(t, "mgmt-server", event["component"])
	}
}

func TestWithComponent_ReplacesAComponentTheLoggerAlreadyCarries(t *testing.T) {
	t.Parallel()
	logs := &syncBuffer{}
	preEnriched := zerolog.New(logs).With().Str("component", "old").Logger()
	server := NewServer(Config{}, WithLogger(preEnriched), WithComponent("new"))

	server.Logger.Info().Msg("probe")

	events := logs.events(t)
	require.Len(t, events, 1)
	assert.Equal(t, "new", events[0]["component"])
}

func TestWithLogger_APreEnrichedLoggerStillWorks(t *testing.T) {
	t.Parallel()
	logs := &syncBuffer{}
	preEnriched := zerolog.New(logs).With().Str("component", "control-server").Logger()
	server := NewServer(Config{}, WithLogger(preEnriched))

	server.Logger.Info().Msg("probe")

	events := logs.events(t)
	require.Len(t, events, 1)
	assert.Equal(t, "control-server", events[0]["component"])
}

func TestWithComponent_LeavesARequestLogWriterAlone(t *testing.T) {
	t.Parallel()
	var access bytes.Buffer
	server := NewServer(Config{}, WithLogWriter(&access), WithComponent("mgmt-server"), WithLogger(zerolog.Nop()))
	server.AddHandlerFunc("GET /ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	rec := httptest.NewRecorder()
	server.handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ping", nil))

	assert.Contains(t, access.String(), "GET /ping", "the combined-format writer still receives the request")
}
