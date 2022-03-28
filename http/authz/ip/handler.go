package ip

import (
	"net/http"

	"github.com/dioad/net/authz"
)

func HandlerFunc(cfg authz.NetworkACLConfig, next http.Handler) http.HandlerFunc {
	authoriser, _ := authz.NewNetworkACL(cfg)
	h := AuthzHandler{next: next, Authoriser: authoriser}
	return h.ServeHTTP
}

type AuthzHandler struct {
	Authoriser *authz.NetworkACL
	next       http.Handler
}

func (h AuthzHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	allowed, err := h.Authoriser.AuthoriseFromString(r.RemoteAddr)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if !allowed {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	h.next.ServeHTTP(w, r.WithContext(r.Context()))
}
