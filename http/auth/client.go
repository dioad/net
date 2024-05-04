package auth

import (
	"github.com/dioad/generics"

	"github.com/dioad/net/http/auth/basic"
	"github.com/dioad/net/http/auth/github"
	"github.com/dioad/net/http/auth/hmac"
)

// TODO: Choose a better name
func AuthClient(authConfig ClientConfig) ClientAuth {
	if !generics.IsZeroValue(authConfig.GitHubAuthConfig) {
		return github.GitHubClientAuth{Config: authConfig.GitHubAuthConfig}
	}
	if !generics.IsZeroValue(authConfig.BasicAuthConfig) {
		return basic.BasicClientAuth{Config: authConfig.BasicAuthConfig}
	}
	if !generics.IsZeroValue(authConfig.HMACAuthConfig) {
		return hmac.ClientAuth{Config: authConfig.HMACAuthConfig}
	}
	return nil
}
