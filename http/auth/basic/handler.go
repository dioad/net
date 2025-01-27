package basic

import (
	stdctx "context"
	"fmt"
	"net/http"

	"github.com/dioad/net/http/auth/context"
)

type Handler struct {
	authMap AuthMap
	config  ServerConfig
}

func (h *Handler) AuthRequest(r *http.Request) (stdctx.Context, error) {
	reqUser, reqPass, _ := r.BasicAuth()

	if reqUser == "" {
		return r.Context(), fmt.Errorf("no credentials provided")
	}

	authenticated, err := h.authMap.Authenticate(reqUser, reqPass)

	if authenticated {
		return context.NewContextWithAuthenticatedPrincipal(r.Context(), reqUser), nil
	}

	if err != nil {
		return r.Context(), err
	}

	return r.Context(), fmt.Errorf("authentication failed")
}

func (h *Handler) Wrap(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, err := h.AuthRequest(r)

		if err != nil {
			if h.config.Realm != "" {
				w.Header().Add("WWW-Authenticate", fmt.Sprintf("Basic realm=\"%s\"", h.config.Realm))
			} else {
				w.Header().Add("WWW-Authenticate", "Basic realm=\"Dioad Connect\"")
			}

			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}

func NewHandler(cfg ServerConfig) (*Handler, error) {
	// TODO: reload from file every x seconds
	// and figure out a way to handle the err
	authMap, err := LoadBasicAuthFromFile(cfg.HTPasswdFile)

	h := &Handler{
		authMap: authMap,
		config:  cfg,
	}

	return h, err
}

func NewHandlerWithMap(authMap AuthMap) (*Handler, error) {
	h := &Handler{
		authMap: authMap,
	}

	return h, nil
}
