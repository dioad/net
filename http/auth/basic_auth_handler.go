package auth

import (
	"context"
	"net/http"
)

type BasicAuthHandler struct {
	handler http.Handler
	authMap BasicAuthMap
}

func BasicAuthHandlerFunc(authMap BasicAuthMap, next http.Handler) http.HandlerFunc {
	h := BasicAuthHandler{handler: next, authMap: authMap}
	return h.ServeHTTP
}

func (h BasicAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reqUser, reqPass, _ := r.BasicAuth()
	principal, err := h.authMap.Authenticate(reqUser, reqPass)
	if !principal || err != nil {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	ctx := context.WithValue(r.Context(), AuthenticatedPrincipal{}, principal)

	h.handler.ServeHTTP(w, r.WithContext(ctx))
}

func NewBasicAuthHandler(handler http.Handler, authMap BasicAuthMap) BasicAuthHandler {
	h := BasicAuthHandler{
		handler: handler,
		authMap: authMap,
	}

	return h
}
