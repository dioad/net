package ip

import (
	stdctx "context"
	"fmt"
	"net/http"

	"github.com/dioad/net/authz"
)

func HandlerFunc(cfg authz.NetworkACLConfig, next http.Handler) http.HandlerFunc {
	h := NewHandler(cfg)
	return h.Wrap(next).ServeHTTP
}

func NewHandler(cfg authz.NetworkACLConfig) *Handler {
	authoriser, _ := authz.NewNetworkACL(cfg)
	return &Handler{Authoriser: authoriser}
}

type Handler struct {
	Authoriser *authz.NetworkACL
}

func (h *Handler) AuthRequest(r *http.Request) (stdctx.Context, error) {
	allowed, err := h.Authoriser.AuthoriseFromString(r.RemoteAddr)
	if err != nil {
		return r.Context(), fmt.Errorf("failed to authorise request: %w", err)
	}

	if !allowed {
		return r.Context(), fmt.Errorf("request not allowed from %s", r.RemoteAddr)
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
