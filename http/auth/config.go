package auth

import (
	"github.com/dioad/net/http/auth/basic"
	"github.com/dioad/net/http/auth/github"
	"github.com/dioad/net/http/auth/hmac"
	"github.com/dioad/net/http/auth/jwt"
)

// ClientConfig represents the authentication configuration for an HTTP client.
type ClientConfig struct {
	BasicAuthConfig  basic.ClientConfig  `mapstructure:"basic"`
	GitHubAuthConfig github.ClientConfig `mapstructure:"github"`
	HMACAuthConfig   hmac.ClientConfig   `mapstructure:"hmac"`
}

// GenericAuthConfig represents a generic authentication configuration.
type GenericAuthConfig struct {
	Name   string                 `mapstructure:"name"`
	Config map[string]interface{} `mapstructure:"config"`
}

// ServerConfig represents the authentication configuration for an HTTP server.
type ServerConfig struct {
	BasicAuthConfig  basic.ServerConfig  `mapstructure:"basic"`
	GitHubAuthConfig github.ServerConfig `mapstructure:"github"`
	HMACAuthConfig   hmac.ServerConfig   `mapstructure:"hmac"`
	JWTAuthConfig    jwt.ServerConfig    `mapstructure:"jwt"`

	Providers []string `mapstructure:"providers"`
}
