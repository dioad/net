package auth

import (
	"github.com/dioad/generics"

	"github.com/dioad/net/http/auth/basic"
	"github.com/dioad/net/http/auth/github"
	"github.com/dioad/net/http/auth/hmac"
)

// NewClientAuth returns a ClientAuth implementation based on the provided configuration.
func NewClientAuth(authConfig ClientConfig) ClientAuth {
	if !generics.IsZeroValue(authConfig.GitHubAuthConfig) {
		return github.ClientAuth{Config: authConfig.GitHubAuthConfig}
	}
	if !generics.IsZeroValue(authConfig.BasicAuthConfig) {
		return basic.ClientAuth{Config: authConfig.BasicAuthConfig}
	}
	if !generics.IsZeroValue(authConfig.HMACAuthConfig) {
		return hmac.ClientAuth{Config: authConfig.HMACAuthConfig}
	}
	return nil
}
