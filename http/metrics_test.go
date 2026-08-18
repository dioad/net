package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestIsWebsocketHandshake(t *testing.T) {
	tests := []struct {
		name       string
		upgrade    string
		connection string
		want       bool
	}{
		{name: "no headers", want: false},
		{name: "upgrade only, no connection", upgrade: "websocket", want: false},
		{name: "connection only, no upgrade", connection: "Upgrade", want: false},
		{name: "valid handshake", upgrade: "websocket", connection: "Upgrade", want: true},
		{name: "case insensitive", upgrade: "WebSocket", connection: "upgrade", want: true},
		{name: "connection with multiple tokens", upgrade: "websocket", connection: "keep-alive, Upgrade", want: true},
		{name: "unrelated upgrade value", upgrade: "h2c", connection: "Upgrade", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.upgrade != "" {
				r.Header.Set("Upgrade", tt.upgrade)
			}
			if tt.connection != "" {
				r.Header.Set("Connection", tt.connection)
			}

			if got := isWebsocketHandshake(r); got != tt.want {
				t.Errorf("isWebsocketHandshake() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetricSet_Middleware_WSLabel(t *testing.T) {
	tests := []struct {
		name       string
		upgrade    string
		connection string
		wantWS     string
	}{
		{name: "plain request", wantWS: "false"},
		{name: "websocket handshake", upgrade: "websocket", connection: "Upgrade", wantWS: "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := prometheus.NewRegistry()
			m := NewMetricSet(registry)

			mux := http.NewServeMux()
			mux.HandleFunc("GET /widgets/{id}", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := m.Middleware(mux, mux)

			r := httptest.NewRequest(http.MethodGet, "/widgets/42", nil)
			if tt.upgrade != "" {
				r.Header.Set("Upgrade", tt.upgrade)
			}
			if tt.connection != "" {
				r.Header.Set("Connection", tt.connection)
			}
			handler.ServeHTTP(httptest.NewRecorder(), r)

			families, err := registry.Gather()
			if err != nil {
				t.Fatalf("Gather() error = %v", err)
			}

			metric := findHistogramMetric(t, families, "dioad_net_http_request_duration_seconds", "route", "GET /widgets/{id}")
			if got := labelValue(metric, "ws"); got != tt.wantWS {
				t.Errorf("ws label = %q, want %q", got, tt.wantWS)
			}
		})
	}
}

func findHistogramMetric(t *testing.T, families []*dto.MetricFamily, name, labelName, labelValueWant string) *dto.Metric {
	t.Helper()

	for _, fam := range families {
		if fam.GetName() != name {
			continue
		}
		for _, metric := range fam.GetMetric() {
			if labelValue(metric, labelName) == labelValueWant {
				return metric
			}
		}
	}

	t.Fatalf("no metric found for family %q with label %s=%s", name, labelName, labelValueWant)
	return nil
}

func labelValue(m *dto.Metric, name string) string {
	for _, lp := range m.GetLabel() {
		if lp.GetName() == name {
			return lp.GetValue()
		}
	}
	return ""
}
