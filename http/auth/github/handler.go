package github

import (
	"net/http"
	"strings"

	"github.com/dioad/net/http/auth/util"

	//"github.com/dioad/net/http/auth"
	"github.com/dioad/net/http/auth/context"
	"github.com/rs/zerolog/log"
)

func GitHubAuthHandlerFunc(cfg GitHubAuthServerConfig, next http.Handler) http.HandlerFunc {
	authenticator := NewGitHubAuthenticator(cfg)
	h := GitHubAuthHandler{next: next, Authenticator: authenticator}
	return h.ServeHTTP
}

type GitHubAuthHandler struct {
	Authenticator *gitHubAuthenticator
	next          http.Handler
}

func (h GitHubAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.TLS == nil {
		http.Error(w, "github auth requires SSL", http.StatusForbidden)
		return
	}

	authHeader := r.Header.Get("Authorization")

	//if authHeader == "" {
	//	http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
	//	return
	//}

	authParts := strings.Split(authHeader, " ")
	if len(authParts) != 2 {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	authType := authParts[0]

	if !(authType == "bearer" || authType == "token") {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	authToken := authParts[1]

	user, err := h.Authenticator.AuthenticateToken(authToken)
	if err != nil {
		w.Header().Add("WWW-Authenticate", "Bearer realm=\"Dioad Connect\"")
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	ctx := context.NewContextWithAuthenticatedPrincipal(r.Context(), user.GetLogin())

	log.Info().Str("principal", user.GetLogin()).Msg("authn")

	userAuthorised := util.IsUserAuthorised(
		user.GetLogin(),
		h.Authenticator.Config.UserAllowList,
		h.Authenticator.Config.UserDenyList)

	log.Info().Str("principal", user.GetLogin()).Bool("authorised", userAuthorised).Msg("authz")

	if !userAuthorised {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	h.next.ServeHTTP(w, r.WithContext(ctx))
}
