package github

import (
	"reflect"

	"github.com/dioad/net/authz"
)

var (
	EmptyGitHubAuthClientConfig = GitHubAuthClientConfig{}
	EmptyGitHubAuthServerConfig = GitHubAuthServerConfig{}
)

// only need ClientID for device flow
type GitHubAuthCommonConfig struct {
	AllowInsecureHTTP bool   `mapstructure:"allow-insecure-http"`
	ClientID          string `mapstructure:"client-id"`
	ClientSecret      string `mapstructure:"client-secret"`

	// ConfigFile containing ClientID and ClientSecret
	ConfigFile string `mapstructure:"config-file"`
}

type GitHubAuthClientConfig struct {
	GitHubAuthCommonConfig           `mapstructure:",squash"`
	AccessToken                      string `mapstructure:"access-token"`
	AccessTokenFile                  string `mapstructure:"access-token-file"`
	EnableAccessTokenFromEnvironment bool   `mapstructure:"enable-access-token-from-environment"`
}

func (c GitHubAuthClientConfig) IsEmpty() bool {
	return reflect.DeepEqual(c, EmptyGitHubAuthClientConfig)
}

type GitHubAuthServerConfig struct {
	GitHubAuthCommonConfig `mapstructure:",squash"`

	PrincipalACLConfig authz.PrincipalACLConfig `mapstructure:"principals"`
}

func (c GitHubAuthServerConfig) IsEmpty() bool {
	return reflect.DeepEqual(c, EmptyGitHubAuthServerConfig)
}
