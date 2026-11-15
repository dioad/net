package http

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dioad/config/configcheck"
)

func TestOperationsConfig_Check(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  OperationsConfig
		want []configcheck.Finding
	}{
		{name: "no management listener", cfg: OperationsConfig{}},
		{name: "loopback", cfg: OperationsConfig{ManagementListenAddress: "127.0.0.1:9000"}},
		{name: "localhost", cfg: OperationsConfig{ManagementListenAddress: "localhost:9000"}},
		{name: "acknowledged non-loopback", cfg: OperationsConfig{ManagementListenAddress: ":9000", ManagementAllowNonLoopback: true}},
		{
			name: "wildcard host without acknowledgement",
			cfg:  OperationsConfig{ManagementListenAddress: ":9000"},
			want: []configcheck.Finding{{
				Path:    "management-listen-address",
				Message: `":9000" is not a loopback address, and the management listener has no authentication; bind 127.0.0.1 or set management-allow-unauthenticated-non-loopback: true to confirm the port is reachable only on a private network`,
			}},
		},
		{
			name: "malformed address is reported once",
			cfg:  OperationsConfig{ManagementListenAddress: "9000"},
			want: []configcheck.Finding{{Path: "management-listen-address", Message: `"9000": address 9000: missing port in address`}},
		},
		{
			name: "acknowledgement without a listener",
			cfg:  OperationsConfig{ManagementAllowNonLoopback: true},
			want: []configcheck.Finding{{
				Path:    "management-allow-unauthenticated-non-loopback",
				Message: "is set but has no effect without management-listen-address, remove it",
			}},
		},
		{
			name: "negative drain delay",
			cfg:  OperationsConfig{ShutdownDrainDelay: -time.Second},
			want: []configcheck.Finding{{Path: "shutdown-drain-delay", Message: "cannot be negative"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := configcheck.NewCollector()
			tc.cfg.Check(c)

			assert.Equal(t, tc.want, c.Findings())
		})
	}
}

func TestOperationsConfig_OperationalServesMetricsAndHonoursDebug(t *testing.T) {
	t.Parallel()

	got := OperationsConfig{ManagementListenAddress: "127.0.0.1:9000", EnableDebug: true}.Operational()

	assert.Equal(t, OperationalConfig{ListenAddress: "127.0.0.1:9000", EnablePrometheusMetrics: true, EnableDebug: true}, got)
}

type readyStub struct{ err error }

func (r readyStub) Ready(context.Context) (any, error) { return "stub", r.err }

func TestNewOperationalServer_ReportsTheConfiguredReadiness(t *testing.T) {
	t.Parallel()

	primary := newPrimary()
	mgmt := NewOperationalServer(primary, OperationalConfig{
		ListenAddress: "127.0.0.1:0",
		Readiness:     readyStub{err: errors.New("database down")},
	})
	addr := serveOn(t, mgmt)

	status, body := getPath(t, addr, "/health/ready")

	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.Contains(t, body, "database down")
	require.NoError(t, primary.AddHealthSource("readiness", readyStub{}), "the source sits under the name readiness on primary")
}
