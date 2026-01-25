package auth

import (
	stdctx "context"
	"net/http"

	"github.com/dioad/generics"

	"github.com/dioad/net/http/auth/basic"
	"github.com/dioad/net/http/auth/github"
	"github.com/dioad/net/http/auth/hmac"
	"github.com/dioad/net/http/auth/jwt"
)

// Handler describes an HTTP authentication handler.
type Handler struct {
	Config     ServerConfig
	middleware Middleware
}

// NewHandler creates a new authentication handler from the provided configuration.
func NewHandler(cfg *ServerConfig) (*Handler, error) {
	mw, err := resolveAuthHandler(cfg)
	if err != nil {
		return nil, err
	}
	return &Handler{
		Config:     *cfg,
		middleware: mw,
	}, nil
}

func resolveAuthHandler(cfg *ServerConfig) (Middleware, error) {
	if !generics.IsZeroValue(cfg.GitHubAuthConfig) {
		return github.NewHandler(cfg.GitHubAuthConfig), nil
	} else if !generics.IsZeroValue(cfg.BasicAuthConfig) {
		return basic.NewHandler(cfg.BasicAuthConfig)
	} else if !generics.IsZeroValue(cfg.HMACAuthConfig) {
		return hmac.NewHandler(cfg.HMACAuthConfig), nil
	} else if !generics.IsZeroValue(cfg.JWTAuthConfig) {
		return jwt.NewHandler(cfg.JWTAuthConfig), nil
	}

	return nil, nil
}

// Wrap wraps an HTTP handler with authentication middleware.
func (h *Handler) Wrap(handler http.Handler) http.Handler {
	if h.middleware == nil {
		return handler
	}

	return h.middleware.Wrap(handler)
}

// HandlerFunc creates an authentication-wrapped HTTP handler function from the provided configuration.
func HandlerFunc(cfg *ServerConfig, origHandler http.HandlerFunc) (http.HandlerFunc, error) {
	h, err := NewHandler(cfg)
	if err != nil {
		return nil, err
	}
	return h.Wrap(origHandler).ServeHTTP, nil
}

// NullHandler is a handler that passes through to the next handler without authentication.
func NullHandler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	}
}

// MultiAuthnHandlerFunc creates a handler function that supports multiple authentication providers.
func MultiAuthnHandlerFunc(cfg *ServerConfig, origHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := origHandler

		// ctx := r.Context()
		var ctx stdctx.Context
		var err error
		for _, provider := range cfg.Providers {
			var a Authenticator
			switch provider {
			case "github":
				a = github.NewHandler(cfg.GitHubAuthConfig)
			case "basic":
				a, err = basic.NewHandler(cfg.BasicAuthConfig)
				if err != nil {
					continue
				}
			case "hmac":
				a = hmac.NewHandler(cfg.HMACAuthConfig)
			case "jwt":
				a = jwt.NewHandler(cfg.JWTAuthConfig)
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
