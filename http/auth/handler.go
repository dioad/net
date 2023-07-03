package auth

import (
	"net/http"

	"github.com/dioad/net/http/auth/basic"
	"github.com/dioad/net/http/auth/github"
	"github.com/dioad/net/http/auth/hmac"
)

func AuthHandlerFunc(cfg *AuthenticationServerConfig, origHandler http.HandlerFunc) http.HandlerFunc {
	h := origHandler

	if !cfg.GitHubAuthConfig.IsEmpty() {
		h = github.GitHubAuthHandlerFunc(cfg.GitHubAuthConfig, h)
	} else if !cfg.BasicAuthConfig.IsEmpty() {
		h = basic.BasicAuthHandlerFunc(cfg.BasicAuthConfig, h)
	} else if !cfg.HMACAuthConfig.IsEmpty() {
		h = hmac.HMACAuthHandlerFunc(cfg.HMACAuthConfig, h)
	}

	return h
}

func NullHandler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	}
}
