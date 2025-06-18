package jwt

import (
	"context"
	"net/http"

	jwtmiddleware "github.com/auth0/go-jwt-middleware/v2"
	"github.com/rs/zerolog"

	"github.com/dioad/net/http/json"
)

type TokenValidator interface {
	ValidateToken(ctx context.Context, tokenString string) (interface{}, error)
}

type Handler struct {
	validator  TokenValidator
	opts       []jwtmiddleware.Option
	logger     zerolog.Logger
	cookieName string
}

func (h *Handler) Wrap(next http.Handler) http.Handler {
	errorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
		jsr := json.NewResponseWithLogger(w, r, h.logger)
		jsr.UnauthorizedWithMessages("unauthorised", err.Error())
	}

	handlerOpts := append(
		[]jwtmiddleware.Option{
			jwtmiddleware.WithErrorHandler(errorHandler),
			jwtmiddleware.WithTokenExtractor(
				jwtmiddleware.MultiTokenExtractor(
					jwtmiddleware.AuthHeaderTokenExtractor,
					jwtmiddleware.CookieTokenExtractor(h.cookieName),
					// GothTokenExtractor()),
				),
			),
		},
		h.opts...,
	)

	middleware := jwtmiddleware.New(
		h.validator.ValidateToken,
		handlerOpts...,
	)

	return middleware.CheckJWT(next)
}

func NewHandler(validator TokenValidator, cookieName string, opts ...jwtmiddleware.Option) *Handler {
	return &Handler{
		cookieName: cookieName,
		validator:  validator,
		opts:       opts,
	}
}

func NewHandlerWithLogger(validator TokenValidator, logger zerolog.Logger, opts ...jwtmiddleware.Option) *Handler {
	return &Handler{
		validator: validator,
		opts:      opts,
		logger:    logger,
	}
}
