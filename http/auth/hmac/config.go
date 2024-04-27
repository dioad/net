package hmac

import "reflect"

var (
	EmptyClientConfig = ClientConfig{}
	EmptyServerConfig = ServerConfig{}
)

type CommonConfig struct {
	AllowInsecureHTTP bool `mapstructure:"allow-insecure-http"`
	// Inline shared key used to HMAC with value from HTTPHeader
	SharedKey string `mapstructure:"shared-key"`
	// Path to read shared key from
	SharedKeyPath string `mapstructure:"shared-key-path"`
	Data          string `mapstructure:"data"`
}

type ClientConfig struct {
	CommonConfig `mapstructure:",squash"`
}

func (c ClientConfig) IsEmpty() bool {
	return reflect.DeepEqual(c, EmptyClientConfig)
}

type ServerConfig struct {
	CommonConfig `mapstructure:",squash"`
	// HTTP Header to use as data input
	PrincipalHeader string `mapstructure:"principal-header"`
}

func (c ServerConfig) IsEmpty() bool {
	return reflect.DeepEqual(c, EmptyServerConfig)
}
