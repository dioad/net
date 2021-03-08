package auth

import (
	"context"
	"net/http"
	"strings"
)

func GitHubAuthHandlerFunc(authenticator *GitHubAuthenticator, next http.Handler) http.HandlerFunc {
	h := GitHubAuthHandler{next: next, Authenticator: authenticator}
	return h.ServeHTTP
}

type GitHubAuthHandler struct {
	Authenticator *GitHubAuthenticator
	next          http.Handler
}

func (h GitHubAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.TLS == nil {
		http.Error(w, "github auth requires SSL", http.StatusForbidden)
		return
	}

	authHeader := r.Header.Get("Authorization")

	if authHeader == "" {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	authParts := strings.Split(authHeader, " ")
	authType := authParts[0]

	if !(authType == "bearer" || authType == "token") {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	authToken := authParts[1]

	user, err := h.Authenticator.AuthenticateToken(authToken)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	ctx := context.WithValue(r.Context(), AuthenticatedPrincipal{}, *user.Login)

	h.next.ServeHTTP(w, r.WithContext(ctx))
}
