package http

import (
	"net"
	"time"

	"github.com/dioad/config/configcheck"
)

// ManagementAllowNonLoopbackKey is the setting that acknowledges a management
// listener bound beyond loopback. OperationsConfig.Check names it in its
// findings, so every service reports the same key.
const ManagementAllowNonLoopbackKey = "management-allow-unauthenticated-non-loopback"

// OperationsConfig holds the operational settings a dioad service takes under
// the same keys: the management listener, pprof, and the shutdown drain. A
// service embeds it squashed, so the keys sit at the top level of its YAML and
// its flags. Check validates it, and ListenAddress, EnableDebug and
// AllowNonLoopback map onto OperationalConfig.
type OperationsConfig struct {
	// ManagementListenAddress is the host:port of the management listener that
	// serves health, /metrics and, with enable-debug, pprof. Empty disables it.
	ManagementListenAddress string `desc:"address of the management listener (health, metrics); empty disables it" mapstructure:"management-listen-address"`

	// ManagementAllowNonLoopback confirms that the management listener may bind
	// beyond loopback. The listener has no authentication, so Check rejects a
	// non-loopback address without it.
	ManagementAllowNonLoopback bool `desc:"allow the unauthenticated management listener to bind a non-loopback address" flag:"management-allow-unauthenticated-non-loopback" mapstructure:"management-allow-unauthenticated-non-loopback"`

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
// acknowledgement, not loopback, an acknowledgement given with no listener to
// apply to, and a negative drain delay. The management listener serves
// health, /metrics and, with enable-debug, pprof, with no authentication and
// no TLS, so binding it beyond loopback has to be a deliberate choice. c is
// positioned at the service's configuration root.
func (o OperationsConfig) Check(c *configcheck.Collector) {
	address := c.Field("management-listen-address")
	switch {
	case o.ManagementListenAddress == "":
		if o.ManagementAllowNonLoopback {
			c.Field(ManagementAllowNonLoopbackKey).Errorf("is set but has no effect without management-listen-address, remove it")
		}
	default:
		configcheck.CheckListenAddress(address, o.ManagementListenAddress)
		if _, _, err := net.SplitHostPort(o.ManagementListenAddress); err != nil {
			break
		}
		if !o.ManagementAllowNonLoopback && !configcheck.IsLoopbackAddress(o.ManagementListenAddress) {
			address.Errorf("%q is not a loopback address, and the management listener has no authentication; "+
				"bind 127.0.0.1 or set %s: true to confirm the port is reachable only on a private network",
				o.ManagementListenAddress, ManagementAllowNonLoopbackKey)
		}
	}
	if o.ShutdownDrainDelay < 0 {
		c.Field("shutdown-drain-delay").Errorf("cannot be negative")
	}
}

// Operational returns the OperationalConfig for the management listener:
// health, /metrics, and /debug when EnableDebug is set. Set Readiness and any
// TLS or PROXY protocol settings on the result.
func (o OperationsConfig) Operational() OperationalConfig {
	return OperationalConfig{
		ListenAddress:           o.ManagementListenAddress,
		EnablePrometheusMetrics: true,
		EnableDebug:             o.EnableDebug,
	}
}
