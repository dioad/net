package auth

import (
	"github.com/dioad/net/http/auth/basic"
	"github.com/dioad/net/http/auth/github"
	"github.com/dioad/net/http/auth/hmac"
)

// need something to deserialize and append details to http.Request
type ClientConfig struct {
	BasicAuthConfig  basic.ClientConfig  `mapstructure:"basic"`
	GitHubAuthConfig github.ClientConfig `mapstructure:"github"`
	HMACAuthConfig   hmac.ClientConfig   `mapstructure:"hmac"`
}

type GenericAuthConfig struct {
	Name   string                 `mapstructure:"name"`
	Config map[string]interface{} `mapstructure:"config"`
}

type ServerConfig struct {
	BasicAuthConfig  basic.ServerConfig  `mapstructure:"basic"`
	GitHubAuthConfig github.ServerConfig `mapstructure:"github"`
	HMACAuthConfig   hmac.ServerConfig   `mapstructure:"hmac"`

	Providers []string `mapstructure:"providers"`
}
