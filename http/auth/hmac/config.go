package hmac

import "time"

// CommonConfig contains shared configuration for HMAC authentication.
type CommonConfig struct {
	AllowInsecureHTTP bool `mapstructure:"allow-insecure-http"`
	// Inline shared key used to HMAC with value from HTTPHeader
	SharedKey string `mapstructure:"shared-key"`
	// Path to read shared key from
	SharedKeyPath string `mapstructure:"shared-key-path"`
	Data          string `mapstructure:"data"`
	// HTTP Headers to include in the HMAC calculation.
	SignedHeaders []string `mapstructure:"signed-headers"`
	// HTTP Header to use for the timestamp (default: X-Timestamp)
	TimestampHeader string `mapstructure:"timestamp-header"`
	// Maximum allowed time difference for the timestamp (default: 5m)
	MaxTimestampDiff time.Duration `mapstructure:"max-timestamp-diff"`
}

// ClientConfig contains configuration for an HMAC authentication client.
type ClientConfig struct {
	CommonConfig `mapstructure:",squash"`
	// Principal ID to use for authentication
	Principal string `mapstructure:"principal"`
}

// ServerConfig contains configuration for an HMAC authentication server.
type ServerConfig struct {
	CommonConfig `mapstructure:",squash"`
}
