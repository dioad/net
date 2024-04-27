package principal

import (
	stdctx "context"
	"fmt"
	"net/http"

	"github.com/dioad/net/authz"
	"github.com/dioad/net/http/auth/context"
	"github.com/dioad/net/http/auth/util"
)

func HandlerFunc(cfg authz.PrincipalACLConfig, next http.Handler) http.HandlerFunc {
	h := NewHandler(cfg)
	return h.Wrap(next).ServeHTTP
}

type Handler struct {
	Config authz.PrincipalACLConfig
}

func NewHandler(cfg authz.PrincipalACLConfig) *Handler {
	return &Handler{Config: cfg}
}

func (h *Handler) AuthRequest(r *http.Request) (stdctx.Context, error) {
	principal := context.AuthenticatedPrincipalFromContext(r.Context())

	if principal == "" {
		return r.Context(), fmt.Errorf("no principal found in context")
	}

	userAuthorised := util.IsUserAuthorised(
		principal,
		h.Config.AllowList,
		h.Config.DenyList)

	if !userAuthorised {
		return r.Context(), fmt.Errorf("user %s is not authorised", principal)
	}
	return r.Context(), nil
}

func (h *Handler) Wrap(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, err := h.AuthRequest(r)

		if err != nil {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}
