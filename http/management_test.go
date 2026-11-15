package http

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dioad/config/configcheck"

	dtls "github.com/dioad/net/tls"
)

func TestManagementConfig_Check(t *testing.T) {
	t.Parallel()

	mutualTLS := dtls.ServerConfig{ClientAuthType: "RequireAndVerifyClientCert", ClientCAFile: "ca.pem"}
	unreachable := func(address string) configcheck.Finding {
		return configcheck.Finding{
			Path: "listen-address",
			Message: `"` + address + `" is reachable from other hosts and the management listener has no authentication; ` +
				"bind to loopback, require client certificates (tls-server client-auth-type RequireAndVerifyClientCert with client-ca-file), " +
				"or set allow-unauthenticated-non-loopback if the network in front of it is trusted",
		}
	}

	tests := []struct {
		name string
		cfg  ManagementConfig
		want []configcheck.Finding
	}{
		{name: "no listener", cfg: ManagementConfig{}},
		{name: "loopback", cfg: ManagementConfig{ListenAddress: "127.0.0.1:9000"}},
		{name: "acknowledged non-loopback", cfg: ManagementConfig{ListenAddress: ":9000", AllowUnauthenticatedNonLoopback: true}},
		{name: "non-loopback that requires client certificates", cfg: ManagementConfig{ListenAddress: ":9000", TLSServer: mutualTLS}},
		{name: "non-loopback without acknowledgement", cfg: ManagementConfig{ListenAddress: ":9000"}, want: []configcheck.Finding{unreachable(":9000")}},
		{
			name: "client certificates requested but not verified against a CA",
			cfg:  ManagementConfig{ListenAddress: ":9000", TLSServer: dtls.ServerConfig{ClientAuthType: "RequireAndVerifyClientCert"}},
			want: []configcheck.Finding{unreachable(":9000")},
		},
		{
			name: "acknowledgement without a listener",
			cfg:  ManagementConfig{AllowUnauthenticatedNonLoopback: true},
			want: []configcheck.Finding{{Path: "allow-unauthenticated-non-loopback", Message: "is set but has no effect without listen-address, remove it"}},
		},
		{
			name: "malformed address is reported once",
			cfg:  ManagementConfig{ListenAddress: "9000"},
			want: []configcheck.Finding{{Path: "listen-address", Message: `"9000": address 9000: missing port in address`}},
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

func TestManagementConfig_OperationalCarriesTheListenerSettings(t *testing.T) {
	t.Parallel()

	got := ManagementConfig{ListenAddress: "127.0.0.1:9000", EnablePrometheusMetrics: true, EnableDebug: true, EnableProxyProtocol: true}.Operational()

	assert.Equal(t, OperationalConfig{ListenAddress: "127.0.0.1:9000", EnablePrometheusMetrics: true, EnableDebug: true, EnableProxyProtocol: true}, got)
}
