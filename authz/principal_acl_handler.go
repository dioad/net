package authz

import (
	"net/http"

	"github.com/dioad/net/http/auth/context"
	"github.com/dioad/net/http/auth/util"
	"github.com/rs/zerolog"
)

func PrincipalACLHandlerFunc(cfg PrincipalACLConfig, logger zerolog.Logger, next http.Handler) http.HandlerFunc {
	h := PrincipalACLHandler{Config: cfg, Logger: logger, next: next}
	return h.ServeHTTP
}

type PrincipalACLHandler struct {
	Config PrincipalACLConfig
	Logger zerolog.Logger
	next   http.Handler
}

func (h PrincipalACLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	principal := context.AuthenticatedPrincipalFromContext(r.Context())

	if principal == "" {
		h.next.ServeHTTP(w, r)
		return
	}

	// TODO: extract from here
	userAuthorised := util.IsUserAuthorised(
		principal,
		h.Config.AllowList,
		h.Config.DenyList)

	h.Logger.Debug().
		Str("principal", principal).
		Bool("authorised", userAuthorised).
		Msg("authz")

	if !userAuthorised {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	h.next.ServeHTTP(w, r)
}
