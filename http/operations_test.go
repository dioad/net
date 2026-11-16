package http

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dioad/config/configcheck"

	dtls "github.com/dioad/net/tls"
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
			name: "non-loopback that requires client certificates",
			cfg: OperationsConfig{ManagementListenAddress: ":9000", ManagementTLSServer: dtls.ServerConfig{
				ClientAuthType: "RequireAndVerifyClientCert", ClientCAFile: "ca.pem",
			}},
		},
		{
			name: "client certificates requested but not verified against a CA",
			cfg: OperationsConfig{ManagementListenAddress: ":9000", ManagementTLSServer: dtls.ServerConfig{
				ClientAuthType: "RequireAndVerifyClientCert",
			}},
			want: []configcheck.Finding{{
				Path:    "management-listen-address",
				Message: `":9000" is not a loopback address, and the management listener has no authentication; bind 127.0.0.1, require client certificates (management-tls-server client-auth-type RequireAndVerifyClientCert with client-ca-file), or set management-allow-unauthenticated-non-loopback: true to confirm the port is reachable only on a private network`,
			}},
		},
		{
			name: "wildcard host without acknowledgement",
			cfg:  OperationsConfig{ManagementListenAddress: ":9000"},
			want: []configcheck.Finding{{
				Path:    "management-listen-address",
				Message: `":9000" is not a loopback address, and the management listener has no authentication; bind 127.0.0.1, require client certificates (management-tls-server client-auth-type RequireAndVerifyClientCert with client-ca-file), or set management-allow-unauthenticated-non-loopback: true to confirm the port is reachable only on a private network`,
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

func TestOperationsConfig_Operational(t *testing.T) {
	t.Parallel()

	off := false
	tests := []struct {
		name string
		cfg  OperationsConfig
		want OperationalConfig
	}{
		{
			name: "metrics are on unless turned off, and debug follows the flag",
			cfg:  OperationsConfig{ManagementListenAddress: "127.0.0.1:9000", EnableDebug: true},
			want: OperationalConfig{ListenAddress: "127.0.0.1:9000", EnablePrometheusMetrics: true, EnableDebug: true},
		},
		{
			name: "metrics can be turned off",
			cfg:  OperationsConfig{ManagementListenAddress: "127.0.0.1:9000", EnablePrometheusMetrics: &off},
			want: OperationalConfig{ListenAddress: "127.0.0.1:9000"},
		},
		{
			name: "PROXY protocol is carried over",
			cfg:  OperationsConfig{ManagementListenAddress: "127.0.0.1:9000", ManagementEnableProxyProtocol: true},
			want: OperationalConfig{ListenAddress: "127.0.0.1:9000", EnablePrometheusMetrics: true, EnableProxyProtocol: true},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, tc.cfg.Operational())
		})
	}
}

type readyStub struct{ err error }

func (r readyStub) Ready(context.Context) (any, error) { return "stub", r.err }

func TestWithReadiness_ReportsReadinessOnTheOperationalServer(t *testing.T) {
	t.Parallel()

	primary := newPrimary()
	mgmt := NewOperationalServer(primary, OperationalConfig{ListenAddress: "127.0.0.1:0"},
		WithReadiness(readyStub{err: errors.New("database down")}))
	addr := serveOn(t, mgmt)

	status, body := getPath(t, addr, "/health/ready")

	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.Contains(t, body, "database down")
	require.NoError(t, primary.AddHealthSource("readiness", readyStub{}), "the source sits under the name readiness on primary")
}

func TestWithReadiness_ServesOnAServerOfItsOwn(t *testing.T) {
	t.Parallel()

	s := NewServer(Config{}, WithLogger(zerolog.Nop()), WithHealth(), WithReadiness(readyStub{err: errors.New("starting")}))
	addr := serveOn(t, s)

	status, body := getPath(t, addr, "/health/ready")

	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.Contains(t, body, "starting")
}

func selfSignedManagementTLS() dtls.ServerConfig {
	return dtls.ServerConfig{SelfSigned: dtls.SelfSignedConfig{
		Subject:  dtls.CertificateSubject{CommonName: "management"},
		SAN:      dtls.SANConfig{IPAddresses: []string{"127.0.0.1"}},
		Duration: "1h",
		Bits:     2048,
	}}
}

func TestOperationsConfig_OperationalWithTLS_UnsetEqualsOperational(t *testing.T) {
	t.Parallel()

	cfg := OperationsConfig{ManagementListenAddress: "127.0.0.1:9000", EnableDebug: true}

	got, err := cfg.OperationalWithTLS(t.Context())

	require.NoError(t, err)
	assert.Equal(t, cfg.Operational(), got)
	assert.Nil(t, got.TLSConfig)
}

func TestOperationsConfig_OperationalWithTLS_ServesOverTLS(t *testing.T) {
	t.Parallel()

	cfg := OperationsConfig{ManagementListenAddress: "127.0.0.1:0", ManagementTLSServer: selfSignedManagementTLS()}
	operational, err := cfg.OperationalWithTLS(t.Context())
	require.NoError(t, err)
	require.NotNil(t, operational.TLSConfig)

	addr := serveOn(t, NewOperationalServer(newPrimary(), operational, WithLogger(zerolog.Nop())))
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}} //nolint:gosec // self-signed test certificate
	t.Cleanup(client.CloseIdleConnections)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://"+addr+"/health/live", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotNil(t, resp.TLS)
}

func TestOperationsConfig_OperationalWithTLS_ReturnsTheTLSError(t *testing.T) {
	t.Parallel()

	tlsCfg := selfSignedManagementTLS()
	tlsCfg.SelfSigned.Duration = "not-a-duration"
	cfg := OperationsConfig{ManagementListenAddress: "127.0.0.1:9000", ManagementTLSServer: tlsCfg}

	got, err := cfg.OperationalWithTLS(t.Context())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "management-tls-server")
	assert.Equal(t, OperationalConfig{}, got)
}
