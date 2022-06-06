package auth

import (
	"github.com/dioad/net/http/auth/basic"
	"github.com/dioad/net/http/auth/github"
	"github.com/dioad/net/http/auth/hmac"
)

// TODO: Choose a better name
func AuthClient(authConfig AuthenticationClientConfig) ClientAuth {
	if !authConfig.GitHubAuthConfig.IsEmpty() {
		return github.GitHubClientAuth{Config: authConfig.GitHubAuthConfig}
	}
	if !authConfig.BasicAuthConfig.IsEmpty() {
		return basic.BasicClientAuth{Config: authConfig.BasicAuthConfig}
	}
	if !authConfig.HMACAuthConfig.IsEmpty() {
		return hmac.HMACClientAuth{Config: authConfig.HMACAuthConfig}
	}
	return nil
}
