package jwt

import (
	"context"
	"net/http"

	jwtmiddleware "github.com/auth0/go-jwt-middleware/v2"

	"github.com/dioad/net/http/json"
)

type TokenValidator interface {
	ValidateToken(ctx context.Context, tokenString string) (interface{}, error)
}

type Handler struct {
	validator TokenValidator
	opts      []jwtmiddleware.Option
}

func (h *Handler) Wrap(next http.Handler) http.Handler {
	errorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
		jsr := json.NewResponse(w)
		jsr.UnauthorizedWithMessages("failed to validate JWT.", err.Error())
	}

	handlerOpts := append(
		[]jwtmiddleware.Option{
			jwtmiddleware.WithErrorHandler(errorHandler),
		},
		h.opts...,
	)

	middleware := jwtmiddleware.New(
		h.validator.ValidateToken,
		handlerOpts...,
	)

	return middleware.CheckJWT(next)
}

func NewHandler(validator TokenValidator, opts ...jwtmiddleware.Option) *Handler {
	return &Handler{
		validator: validator,
		opts:      opts,
	}
}
