package handler

import (
	"net/http"

	dch "github.com/dioad/net/http"
)

type BasicAuthHandler struct {
	handler http.Handler
	authMap dch.BasicAuthMap
}

func (h BasicAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reqUser, reqPass, _ := r.BasicAuth()
	result, err := h.authMap.Authenticate(reqUser, reqPass)
	if !result || err != nil {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	h.handler.ServeHTTP(w, r)
}

func NewBasicAuthHandler(handler http.HandlerFunc, authMap dch.BasicAuthMap) BasicAuthHandler {
	h := BasicAuthHandler{
		handler: handler,
		authMap: authMap,
	}

	return h
}
