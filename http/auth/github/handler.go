package github

import (
	stdctx "context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	// "github.com/dioad/net/http/auth"
	"github.com/dioad/net/http/auth/context"
)

func NewHandler(cfg ServerConfig) *Handler {
	return &Handler{
		Authenticator: NewGitHubAuthenticator(cfg),
	}
}

type Handler struct {
	Authenticator *Authenticator
}

func (h *Handler) AuthRequest(r *http.Request) (stdctx.Context, error) {
	authHeader := r.Header.Get("Authorization")

	authParts := strings.Split(authHeader, " ")
	if len(authParts) != 2 {
		return r.Context(), errors.New("invalid auth header")
	}

	authType := strings.ToLower(authParts[0])

	if authType != "bearer" && authType != "token" {
		return r.Context(), errors.New("invalid auth type")
	}

	authToken := authParts[1]

	user, err := h.Authenticator.AuthenticateToken(authToken)
	if err != nil {
		return r.Context(), fmt.Errorf("failed to authenticate token: %w", err)
	}

	ctx := context.NewContextWithAuthenticatedPrincipal(r.Context(), user.Login)

	ctx = NewContextWithGitHubUserInfo(ctx, user)

	log.Debug().
		Str("principal", user.Login).
		Str("email", user.PrimaryEmail).
		Str("company", user.Company).
		Msg("authn")

	return ctx, nil
}

func (h *Handler) Wrap(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, err := h.AuthRequest(r)

		if err != nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}
