package auth

import (
	"net/http"
	"reflect"

	"github.com/dioad/net/http/auth/basic"
	"github.com/dioad/net/http/auth/github"
	"github.com/dioad/net/http/auth/hmac"
)

func AuthHandlerFunc(cfg *AuthenticationServerConfig, origHandler http.HandlerFunc) http.HandlerFunc {
	h := origHandler

	if !reflect.DeepEqual(cfg.GitHubAuthConfig, github.EmptyGitHubAuthServerConfig) {
		h = github.GitHubAuthHandlerFunc(cfg.GitHubAuthConfig, h)
	} else if !reflect.DeepEqual(cfg.BasicAuthConfig, basic.EmptyBasicAuthServerConfig) {
		h = basic.BasicAuthHandlerFunc(cfg.BasicAuthConfig, h)
	} else if !reflect.DeepEqual(cfg.HMACAuthConfig, hmac.EmptyHMACAuthServerConfig) {
		h = hmac.HMACAuthHandlerFunc(cfg.HMACAuthConfig, h)
	}

	return h
}
