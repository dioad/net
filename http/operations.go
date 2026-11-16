package http

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/dioad/config/configcheck"

	dtls "github.com/dioad/net/tls"
)

// ManagementAllowNonLoopbackKey is the setting that acknowledges a management
// listener bound beyond loopback. OperationsConfig.Check names it in its
// findings, so every service reports the same key.
const ManagementAllowNonLoopbackKey = "management-allow-unauthenticated-non-loopback"

// OperationsConfig holds the operational settings a dioad service takes under
// the same keys: the management listener, pprof, and the shutdown drain. A
// service embeds it squashed, so the keys sit at the top level of its YAML and
// its flags. Check validates it, and Operational maps it onto
// OperationalConfig.
type OperationsConfig struct {
	// ManagementListenAddress is the host:port of the management listener that
	// serves health, /metrics and, with enable-debug, pprof. Empty disables it.
	ManagementListenAddress string `desc:"address of the management listener (health, metrics); empty disables it" mapstructure:"management-listen-address"`

	// ManagementAllowNonLoopback confirms that the management listener may bind
	// beyond loopback with nothing in front of it to authenticate callers. Check
	// accepts a non-loopback address without it only when ManagementTLSServer
	// requires client certificates.
	ManagementAllowNonLoopback bool `desc:"allow the unauthenticated management listener to bind a non-loopback address" flag:"management-allow-unauthenticated-non-loopback" mapstructure:"management-allow-unauthenticated-non-loopback"`

	// ManagementEnableProxyProtocol accepts a PROXY protocol header on the
	// management listener, for one a proxy forwards to. Leave it off for a
	// platform check or scraper that connects directly.
	ManagementEnableProxyProtocol bool `desc:"accept a PROXY protocol header on the management listener" mapstructure:"management-enable-proxy-protocol"`

	// ManagementTLSServer serves the management listener over TLS. With
	// client-auth-type RequireAndVerifyClientCert and a client-ca-file it also
	// authenticates callers, which lets the listener bind beyond loopback
	// without the acknowledgement. OperationalWithTLS converts it into the
	// TLSConfig of the OperationalConfig it returns.
	ManagementTLSServer dtls.ServerConfig `desc:"TLS settings of the management listener" mapstructure:"management-tls-server"`

	// EnablePrometheusMetrics serves /metrics on the management listener. Unset
	// means on: the listener exists for health and metrics, and a service has to
	// set it false to turn them off.
	EnablePrometheusMetrics *bool `desc:"serve Prometheus metrics on the management listener (default true)" mapstructure:"enable-prometheus-metrics"`

	// EnableDebug should only be true in development environments. Set to false
	// in production to avoid exposing sensitive information.
	EnableDebug bool `desc:"expose pprof and verbose diagnostics; development only" mapstructure:"enable-debug"`

	// ShutdownDrainDelay is how long shutdown waits, after /health/ready starts
	// reporting not-ready and before listeners close, so a load balancer stops
	// routing new requests first. Set it to at least one probe interval. Zero
	// (the default) closes listeners immediately.
	ShutdownDrainDelay time.Duration `desc:"how long shutdown waits after readiness fails before closing listeners" mapstructure:"shutdown-drain-delay"`
}

// Check reports to c a management listener that is malformed or, without the
// acknowledgement, not loopback and not protected by client certificates, an
// acknowledgement given with no listener to apply to, a TLS block that does not
// check out, and a negative drain delay. The management listener serves
// health, /metrics and, with enable-debug, pprof, with no authentication of its
// own, so binding it beyond loopback has to be a deliberate choice. c is
// positioned at the service's configuration root.
func (o OperationsConfig) Check(c *configcheck.Collector) {
	o.ManagementTLSServer.Check(c.Field("management-tls-server"))

	address := c.Field("management-listen-address")
	switch o.ManagementListenAddress {
	case "":
		if o.ManagementAllowNonLoopback {
			c.Field(ManagementAllowNonLoopbackKey).Errorf("is set but has no effect without management-listen-address, remove it")
		}
	default:
		configcheck.CheckListenAddress(address, o.ManagementListenAddress)
		if _, _, err := net.SplitHostPort(o.ManagementListenAddress); err != nil {
			break
		}
		if !o.ManagementAllowNonLoopback && !configcheck.IsLoopbackAddress(o.ManagementListenAddress) &&
			!o.ManagementTLSServer.RequiresClientCertificate() {
			address.Errorf("%q is not a loopback address, and the management listener has no authentication; "+
				"bind 127.0.0.1, require client certificates (management-tls-server client-auth-type RequireAndVerifyClientCert with client-ca-file), "+
				"or set %s: true to confirm the port is reachable only on a private network",
				o.ManagementListenAddress, ManagementAllowNonLoopbackKey)
		}
	}
	if o.ShutdownDrainDelay < 0 {
		c.Field("shutdown-drain-delay").Errorf("cannot be negative")
	}
}

// Operational returns the OperationalConfig for the management listener:
// health, /metrics unless turned off, and /debug when EnableDebug is set. Pass
// WithReadiness to NewOperationalServer so /health/ready reports the service's
// readiness. It leaves TLSConfig unset: use OperationalWithTLS to serve the
// listener over ManagementTLSServer.
func (o OperationsConfig) Operational() OperationalConfig {
	return OperationalConfig{
		ListenAddress:           o.ManagementListenAddress,
		EnablePrometheusMetrics: o.EnablePrometheusMetrics == nil || *o.EnablePrometheusMetrics,
		EnableDebug:             o.EnableDebug,
		EnableProxyProtocol:     o.ManagementEnableProxyProtocol,
	}
}

// OperationalWithTLS is Operational with ManagementTLSServer converted into
// TLSConfig. With ManagementTLSServer unset it equals Operational. It returns
// the error from building the TLS configuration, so a service never serves the
// management listener in plaintext while its configuration asks for TLS.
func (o OperationsConfig) OperationalWithTLS(ctx context.Context) (OperationalConfig, error) {
	operational := o.Operational()

	tlsConfig, err := dtls.NewServerTLSConfig(ctx, o.ManagementTLSServer)
	if err != nil {
		return OperationalConfig{}, fmt.Errorf("management-tls-server: %w", err)
	}
	operational.TLSConfig = tlsConfig

	return operational, nil
}
