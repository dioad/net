package auth

import (
	"net/http"
	"reflect"
)

func AuthHandlerFunc(cfg *AuthenticationServerConfig, origHandler http.HandlerFunc) http.HandlerFunc {
	h := origHandler

	if !reflect.DeepEqual(cfg.GitHubAuthConfig, EmptyGitHubAuthServerConfig) {
		h = GitHubAuthHandlerFunc(cfg.GitHubAuthConfig, h)
	} else if !reflect.DeepEqual(cfg.BasicAuthConfig, EmptyBasicAuthServerConfig) {
		h = BasicAuthHandlerFunc(cfg.BasicAuthConfig, h)
	} else if !reflect.DeepEqual(cfg.HMACAuthConfig, EmptyHMACAuthServerConfig) {
		h = HMACAuthHandlerFunc(cfg.HMACAuthConfig, h)
	}

	return h
}
