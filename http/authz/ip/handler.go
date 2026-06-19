// Package ip provides IP-based authorization middleware.
package ip

import (
	stdctx "context"
	"fmt"
	"net/http"

	"github.com/dioad/net/authz"
)

// HandlerFunc creates an IP-based authorization-wrapped HTTP handler function.
func HandlerFunc(cfg authz.NetworkACLConfig, next http.Handler) (http.HandlerFunc, error) {
	h, err := NewHandler(cfg)
	if err != nil {
		return nil, err
	}
	return h.Wrap(next).ServeHTTP, nil
}

// NewHandler creates a new IP-based authorization handler.
func NewHandler(cfg authz.NetworkACLConfig) (*Handler, error) {
	authoriser, err := authz.NewNetworkACL(cfg)
	if err != nil {
		return nil, fmt.Errorf("ip.NewHandler: invalid ACL config: %w", err)
	}
	return &Handler{Authoriser: authoriser}, nil
}

// Handler implements IP-based authorization for HTTP servers.
type Handler struct {
	Authoriser authz.Authoriser
}

// AuthRequest checks if an HTTP request is authorized based on the remote IP address.
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

// Wrap wraps an HTTP handler with IP-based authorization middleware.
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
