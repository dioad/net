package hmac

import "reflect"

var (
	EmptyHMACAuthClientConfig = HMACAuthClientConfig{}
	EmptyHMACAuthServerConfig = HMACAuthServerConfig{}
)

type HMACAuthCommonConfig struct {
	AllowInsecureHTTP bool `mapstructure:"allow-insecure-http"`
	// Inline shared key used to HMAC with value from HTTPHeader
	SharedKey string `mapstructure:"shared-key"`
	// Path to read shared key from
	SharedKeyPath string `mapstructure:"shared-key-path"`
	Data          string `mapstructure:"data"`
}

type HMACAuthClientConfig struct {
	HMACAuthCommonConfig `mapstructure:",squash"`
}

func (c HMACAuthClientConfig) IsEmpty() bool {
	return reflect.DeepEqual(c, EmptyHMACAuthClientConfig)
}

type HMACAuthServerConfig struct {
	HMACAuthCommonConfig `mapstructure:",squash"`
	// HTTP Header to use as data input
	HTTPHeader string `mapstructure:"http-header"`
}

func (c HMACAuthServerConfig) IsEmpty() bool {
	return reflect.DeepEqual(c, EmptyHMACAuthServerConfig)
}
