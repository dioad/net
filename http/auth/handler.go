package auth

import (
	stdctx "context"
	"net/http"

	"github.com/dioad/generics"

	"github.com/dioad/net/http/auth/basic"
	"github.com/dioad/net/http/auth/github"
	"github.com/dioad/net/http/auth/hmac"
)

type Handler struct {
	Config ServerConfig
}

func NewHandler(cfg *ServerConfig) *Handler {
	return &Handler{
		Config: *cfg,
	}
}

func (h *Handler) Wrap(handler http.Handler) http.Handler {
	var mw Middleware
	if !generics.IsZeroValue(h.Config.GitHubAuthConfig) {
		mw = github.NewHandler(h.Config.GitHubAuthConfig)
	} else if !generics.IsZeroValue(h.Config.BasicAuthConfig) {
		mw = basic.NewHandler(h.Config.BasicAuthConfig)
	} else if !generics.IsZeroValue(h.Config.HMACAuthConfig) {
		mw = hmac.NewHandler(h.Config.HMACAuthConfig)
	}

	if mw == nil {
		return handler
	}

	return mw.Wrap(handler)
}

func HandlerFunc(cfg *ServerConfig, origHandler http.HandlerFunc) http.HandlerFunc {
	return NewHandler(cfg).Wrap(origHandler).ServeHTTP
}

func NullHandler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	}
}

func MultiAuthnHandlerFunc(cfg *ServerConfig, origHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := origHandler

		// ctx := r.Context()
		var ctx stdctx.Context
		var err error = nil
		for _, provider := range cfg.Providers {
			var a Authenticator
			switch provider {
			case "github":
				a = github.NewHandler(cfg.GitHubAuthConfig)
			case "basic":
				a = basic.NewHandler(cfg.BasicAuthConfig)
			case "hmac":
				a = hmac.NewHandler(cfg.HMACAuthConfig)
			}

			ctx, err = a.AuthRequest(r)
			if err == nil {
				r = r.WithContext(ctx)
				break
			}
		}

		if err != nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		h.ServeHTTP(w, r)
	}
}
