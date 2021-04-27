package auth

import (
	"reflect"

	"github.com/dioad/net/http/auth/basic"
	"github.com/dioad/net/http/auth/github"
	"github.com/dioad/net/http/auth/hmac"
)

// TODO: Choose a better name
func AuthClient(authConfig AuthenticationClientConfig) ClientAuth {
	if !reflect.DeepEqual(authConfig.GitHubAuthConfig, github.EmptyGitHubAuthClientConfig) {
		return github.GitHubClientAuth{Config: authConfig.GitHubAuthConfig}
	} else if !reflect.DeepEqual(authConfig.BasicAuthConfig, basic.EmptyBasicAuthClientConfig) {
		return basic.BasicClientAuth{Config: authConfig.BasicAuthConfig}
	} else if !reflect.DeepEqual(authConfig.HMACAuthConfig, hmac.EmptyHMACAuthClientConfig) {
		return hmac.HMACClientAuth{Config: authConfig.HMACAuthConfig}
	}
	return nil
}
