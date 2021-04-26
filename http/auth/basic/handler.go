package basic

import (
	"net/http"

	"github.com/dioad/net/http/auth"
)

type BasicAuthHandler struct {
	handler http.Handler
	authMap BasicAuthMap
}

func BasicAuthHandlerFunc(cfg BasicAuthServerConfig, next http.Handler) http.HandlerFunc {
	h := NewBasicAuthHandler(next, cfg)
	return h.ServeHTTP
}

func (h BasicAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.TLS == nil {
		http.Error(w, "basic auth requires SSL", http.StatusForbidden)
		return
	}

	reqUser, reqPass, _ := r.BasicAuth()
	authenticated, err := h.authMap.Authenticate(reqUser, reqPass)
	if !authenticated || err != nil {
		w.Header().Add("WWW-Authenticate", "Basic realm=\"Dioad Connect\"")
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	ctx := auth.NewContextWithAuthenticatedPrincipal(r.Context(), reqUser)

	h.handler.ServeHTTP(w, r.WithContext(ctx))
}

func NewBasicAuthHandler(handler http.Handler, cfg BasicAuthServerConfig) BasicAuthHandler {
	authMap, _ := LoadBasicAuthFromFile(cfg.HTPasswdFile)

	h := BasicAuthHandler{
		handler: handler,
		authMap: authMap,
	}

	return h
}

func NewBasicAuthHandlerWithMap(handler http.Handler, authMap BasicAuthMap) BasicAuthHandler {
	h := BasicAuthHandler{
		handler: handler,
		authMap: authMap,
	}

	return h
}
