package github

import (
	"reflect"

	"github.com/mitchellh/mapstructure"

	"github.com/dioad/net/authz"
)

var (
	EmptyClientConfig = ClientConfig{}
	EmptyServerConfig = ServerConfig{}
)

// only need ClientID for device flow
type CommonConfig struct {
	AllowInsecureHTTP bool   `mapstructure:"allow-insecure-http"`
	ClientID          string `mapstructure:"client-id"`
	ClientSecret      string `mapstructure:"client-secret"`

	// ConfigFile containing ClientID and ClientSecret
	ConfigFile string `mapstructure:"config-file"`
}

type ClientConfig struct {
	CommonConfig                     `mapstructure:",squash"`
	AccessToken                      string `mapstructure:"access-token"`
	AccessTokenFile                  string `mapstructure:"access-token-file"`
	EnableAccessTokenFromEnvironment bool   `mapstructure:"enable-access-token-from-environment"`
}

func (c ClientConfig) IsEmpty() bool {
	return reflect.DeepEqual(c, EmptyClientConfig)
}

type ServerConfig struct {
	CommonConfig `mapstructure:",squash"`

	PrincipalACLConfig authz.PrincipalACLConfig `mapstructure:"principals"`
}

func (c ServerConfig) IsEmpty() bool {
	return reflect.DeepEqual(c, EmptyServerConfig)
}

func FromMap(m map[string]interface{}) ServerConfig {
	var c ServerConfig
	_ = mapstructure.Decode(m, &c)
	return c
}
