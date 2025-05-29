package hmac

import (
	stdcontext "context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/dioad/net/http/auth/context"
)

func NewHandler(cfg ServerConfig) *Handler {
	return &Handler{cfg: cfg}
}

type Handler struct {
	cfg ServerConfig
}

func (a *Handler) AuthRequest(r *http.Request) (stdcontext.Context, error) {
	sharedKey := a.cfg.SharedKey
	principalHeader := a.cfg.PrincipalHeader

	authHeader := r.Header.Get("Authorization")
	authPrincipal := r.Header.Get(principalHeader)

	if authHeader == "" {
		return r.Context(), errors.New("missing auth header")
	}

	if authPrincipal == "" {
		return r.Context(), errors.New("missing principal header")
	}

	authParts := strings.Split(authHeader, " ")
	authType := authParts[0]
	authToken := authParts[1]

	if authType != "bearer" && authType != "token" {
		return r.Context(), errors.New("invalid auth type")
	}

	verificationKey, err := HMACKey([]byte(sharedKey), []byte(authPrincipal))
	if err != nil {
		return r.Context(), fmt.Errorf("failed to generate verification key: %w", err)
	}

	// Check build keys.
	// TODO: change to use hmac.Equal if ! hmac.Equal(agentInfo.AuthKey, key) {
	if authToken != verificationKey {
		// http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return r.Context(), errors.New("invalid auth token")
	}

	ctx := context.NewContextWithAuthenticatedPrincipal(r.Context(), authPrincipal)

	return ctx, nil
}

func (a *Handler) Wrap(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, err := a.AuthRequest(r)

		if err != nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}
