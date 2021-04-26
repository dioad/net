package hmac

var (
	EmptyHMACAuthClientConfig = HMACAuthClientConfig{}
	EmptyHMACAuthServerConfig = HMACAuthServerConfig{}
)

type HMACAuthCommonConfig struct {
	// Inline shared key used to HMAC with value from HTTPHeader
	SharedKey string `mapstructure:"shared-key"`
	// Path to read shared key from
	SharedKeyPath string `mapstructure:"shared-key-path"`
}

type HMACAuthClientConfig struct {
	HMACAuthCommonConfig `mapstructure:",squash"`
}

type HMACAuthServerConfig struct {
	HMACAuthCommonConfig `mapstructure:",squash"`
	// HTTP Header to use as data input
	HTTPHeader string `mapstructure:"http-header"`
}
