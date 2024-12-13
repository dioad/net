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
	Config     ServerConfig
	middleware Middleware
}

func NewHandler(cfg *ServerConfig) *Handler {
	return &Handler{
		Config:     *cfg,
		middleware: resolveAuthHandler(cfg),
	}
}

func resolveAuthHandler(cfg *ServerConfig) Middleware {
	if !generics.IsZeroValue(cfg.GitHubAuthConfig) {
		return github.NewHandler(cfg.GitHubAuthConfig)
	} else if !generics.IsZeroValue(cfg.BasicAuthConfig) {
		return basic.NewHandler(cfg.BasicAuthConfig)
	} else if !generics.IsZeroValue(cfg.HMACAuthConfig) {
		return hmac.NewHandler(cfg.HMACAuthConfig)
	}

	return nil
}

func (h *Handler) Wrap(handler http.Handler) http.Handler {
	if h.middleware == nil {
		return handler
	}

	return h.middleware.Wrap(handler)
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
