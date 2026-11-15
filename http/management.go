package http

import (
	"net"

	"github.com/dioad/config/configcheck"

	dtls "github.com/dioad/net/tls"
)

// ManagementConfig is the settings of a management listener a service takes
// under one nested block, usually "management". It serves health, /metrics and,
// with enable-debug, pprof, and has no authentication of its own. Check
// validates it and Operational maps it onto OperationalConfig. A service with
// settings of its own for the listener embeds ManagementConfig squashed in its
// own block type. OperationsConfig is the flat form of the same listener for
// services that have no such block.
type ManagementConfig struct {
	// ListenAddress is the host:port of the listener. Empty means no listener,
	// which a service that always runs one reports itself.
	ListenAddress string `desc:"address of the management listener" mapstructure:"listen-address"`

	// AllowUnauthenticatedNonLoopback confirms that the listener may bind beyond
	// loopback with nothing in front of it to authenticate callers. Check
	// accepts a non-loopback address without it only when TLSServer requires
	// client certificates.
	AllowUnauthenticatedNonLoopback bool `desc:"allow the unauthenticated management listener to bind a non-loopback address" mapstructure:"allow-unauthenticated-non-loopback"`

	// EnablePrometheusMetrics serves /metrics.
	EnablePrometheusMetrics bool `desc:"serve Prometheus metrics" mapstructure:"enable-prometheus-metrics"`

	// EnableDebug serves pprof. It exposes memory contents, so keep it to
	// development.
	EnableDebug bool `desc:"expose pprof and verbose diagnostics; development only" mapstructure:"enable-debug"`

	// EnableProxyProtocol accepts a PROXY protocol header, for a listener a
	// proxy forwards to.
	EnableProxyProtocol bool `desc:"accept a PROXY protocol header" mapstructure:"enable-proxy-protocol"`

	// TLSServer serves the listener over TLS. With client-auth-type
	// RequireAndVerifyClientCert and a client-ca-file it also authenticates
	// callers, which lets the listener bind beyond loopback unacknowledged.
	TLSServer dtls.ServerConfig `desc:"TLS settings of the management listener" mapstructure:"tls-server"`
}

// Check reports to c a listener that is malformed, one that is not loopback
// with nothing to authenticate its callers, an acknowledgement given with no
// listener to apply to, and a TLS block that does not check out. A listener is
// authenticated when TLSServer requires client certificates. c is positioned at
// the management block.
func (m ManagementConfig) Check(c *configcheck.Collector) {
	m.TLSServer.Check(c.Field("tls-server"))

	address := c.Field("listen-address")
	if m.ListenAddress == "" {
		if m.AllowUnauthenticatedNonLoopback {
			c.Field("allow-unauthenticated-non-loopback").Errorf("is set but has no effect without listen-address, remove it")
		}
		return
	}

	configcheck.CheckListenAddress(address, m.ListenAddress)
	if _, _, err := net.SplitHostPort(m.ListenAddress); err != nil {
		return
	}
	if configcheck.IsLoopbackAddress(m.ListenAddress) || m.AllowUnauthenticatedNonLoopback || m.TLSServer.RequiresClientCertificate() {
		return
	}
	address.Errorf("%q is reachable from other hosts and the management listener has no authentication; "+
		"bind to loopback, require client certificates (tls-server client-auth-type RequireAndVerifyClientCert with client-ca-file), "+
		"or set allow-unauthenticated-non-loopback if the network in front of it is trusted", m.ListenAddress)
}

// Operational returns the OperationalConfig for the listener. Set TLSConfig on
// the result from TLSServer, and EnableStatus where the service has a status
// resource.
func (m ManagementConfig) Operational() OperationalConfig {
	return OperationalConfig{
		ListenAddress:           m.ListenAddress,
		EnablePrometheusMetrics: m.EnablePrometheusMetrics,
		EnableDebug:             m.EnableDebug,
		EnableProxyProtocol:     m.EnableProxyProtocol,
	}
}
