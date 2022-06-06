package auth

import (
	"reflect"

	"github.com/dioad/net/http/auth/basic"
	"github.com/dioad/net/http/auth/github"
	"github.com/dioad/net/http/auth/hmac"
)

var (
	EmptyAuthenticationClientConfig = AuthenticationClientConfig{}
	EmptyAuthenticationServerConfig = AuthenticationServerConfig{}
)

// need something to deserialize and append details to http.Request
type AuthenticationClientConfig struct {
	BasicAuthConfig  basic.BasicAuthClientConfig   `mapstructure:"basic"`
	GitHubAuthConfig github.GitHubAuthClientConfig `mapstructure:"github"`
	HMACAuthConfig   hmac.HMACAuthClientConfig     `mapstructure:"hmac"`
}

func (c AuthenticationClientConfig) IsEmpty() bool {
	return reflect.DeepEqual(c, EmptyAuthenticationClientConfig)
}

type AuthenticationServerConfig struct {
	BasicAuthConfig  basic.BasicAuthServerConfig   `mapstructure:"basic"`
	GitHubAuthConfig github.GitHubAuthServerConfig `mapstructure:"github"`
	HMACAuthConfig   hmac.HMACAuthServerConfig     `mapstructure:"hmac"`
}

func (c AuthenticationServerConfig) IsEmpty() bool {
	return reflect.DeepEqual(c, EmptyAuthenticationServerConfig)
}
