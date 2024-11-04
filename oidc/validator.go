package oidc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/auth0/go-jwt-middleware/v2/jwks"
	jwtvalidator "github.com/auth0/go-jwt-middleware/v2/validator"
	"github.com/rs/zerolog"
)

type ValidatorDebugger struct {
	logger    zerolog.Logger
	validator TokenValidator
}

type TokenValidator interface {
	ValidateToken(ctx context.Context, tokenString string) (interface{}, error)
}

func WithLogger(logger zerolog.Logger) func(*ValidatorDebugger) {
	return func(v *ValidatorDebugger) {
		v.logger = logger
	}
}

func WithLabel(key, value string) func(*ValidatorDebugger) {
	return func(v *ValidatorDebugger) {
		v.logger = v.logger.With().Str(key, value).Logger()
	}
}

type ValidatorDebugOpts func(*ValidatorDebugger)

func NewValidatorDebugger(validator TokenValidator, opts ...ValidatorDebugOpts) *ValidatorDebugger {
	v := &ValidatorDebugger{
		validator: validator,
	}

	for _, o := range opts {
		o(v)
	}

	return v
}

func decodeTokenData(accessToken string) (interface{}, error) {
	// Decode Access Token and extract expiry and any other details necessary from the token
	tokenParts := strings.Split(accessToken, ".")
	if len(tokenParts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(tokenParts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode token payload: %w", err)
	}

	var tokenData map[string]interface{}
	if err := json.Unmarshal(payload, &tokenData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token payload: %w", err)
	}

	var tokenDataExpiry time.Time
	if expiry, ok := tokenData["exp"].(float64); ok {
		tokenDataExpiry = time.Unix(int64(expiry), 0)
		tokenData["exp_datetime"] = tokenDataExpiry
	}

	var tokenDataIssuedAt time.Time
	if issuedAt, ok := tokenData["iat"].(float64); ok {
		tokenDataIssuedAt = time.Unix(int64(issuedAt), 0)
		tokenData["iat_datetime"] = tokenDataIssuedAt
	}

	var tokenDataNotBefore time.Time
	if notBefore, ok := tokenData["nbf"].(float64); ok {
		tokenDataNotBefore = time.Unix(int64(notBefore), 0)
		tokenData["nbf_datetime"] = tokenDataNotBefore
	}

	return tokenData, nil
}

func (v *ValidatorDebugger) ValidateToken(ctx context.Context, tokenString string) (interface{}, error) {
	tokenDetails, err := decodeTokenData(tokenString)
	if err != nil {
		return nil, fmt.Errorf("error decoding token data: %w", err)
	}

	v.logger.Debug().Interface("DecodedToken", tokenDetails).Msg("decoded token")

	claims, err := v.validator.ValidateToken(ctx, tokenString)
	if err != nil {
		v.logger.Error().Err(err).Msg("error validating token")
	}
	return claims, err
}

type MultiValidator struct {
	validators []TokenValidator
}

func (v *MultiValidator) ValidateToken(ctx context.Context, tokenString string) (interface{}, error) {
	var err error
	var claims interface{}
	var errs []string
	for _, vtor := range v.validators {
		claims, err = vtor.ValidateToken(ctx, tokenString)
		if err == nil {
			return claims, nil
		}

		errs = append(errs, err.Error())
	}

	return nil, fmt.Errorf("token validation failed: %w", err)
}

func NewMultiValidator(validators ...TokenValidator) *MultiValidator {
	return &MultiValidator{validators: validators}
}

func NewMultiValidatorFromConfig(configs []ValidatorConfig, opts ...jwtvalidator.Option) (*MultiValidator, error) {
	validators, err := NewValidatorsFromConfig(configs, opts...)
	if err != nil {
		return nil, fmt.Errorf("error creating validators from config: %w", err)
	}
	return NewMultiValidator(validators...), nil
}

func NewValidatorFromConfig(config *ValidatorConfig, opts ...jwtvalidator.Option) (TokenValidator, error) {
	var validator TokenValidator
	endpoint, err := NewEndpointFromConfig(&config.EndpointConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating endpoint from config: %w", err)
	}

	cacheTTL := 5 * time.Minute
	if config.CacheTTL != 0 {
		cacheTTL = time.Duration(config.CacheTTL) * time.Second
	}

	signatureAlgorithm := jwtvalidator.RS256
	if config.SignatureAlgorithm != "" {
		signatureAlgorithm = jwtvalidator.SignatureAlgorithm(config.SignatureAlgorithm)
	}

	issuerURL := config.URL
	if config.Issuer != "" {
		issuerURL = config.Issuer
	}

	allowedClockSkew := time.Minute
	if config.AllowedClockSkew != 0 {
		allowedClockSkew = time.Duration(config.AllowedClockSkew) * time.Second
	}

	opts = append([]jwtvalidator.Option{
		jwtvalidator.WithAllowedClockSkew(allowedClockSkew),
	}, opts...)

	provider := jwks.NewCachingProvider(endpoint.URL(), cacheTTL)

	validator, err = jwtvalidator.New(
		provider.KeyFunc,
		signatureAlgorithm,
		issuerURL,
		config.Audiences,
		opts...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to configure validator for %v: %w", config.URL, err)
	}

	if config.Debug {
		validator = NewValidatorDebugger(validator,
			WithLabel("issuer", issuerURL),
			WithLabel("type", config.Type),
			WithLabel("audiences", strings.Join(config.Audiences, ",")),
			WithLabel("signatureAlgorithm", config.SignatureAlgorithm),
			WithLabel("allowedClockSkew", allowedClockSkew.String()),
			WithLabel("cacheTTL", cacheTTL.String()),
		)
	}

	return validator, nil
}

func NewValidatorsFromConfig(configs []ValidatorConfig, opts ...jwtvalidator.Option) ([]TokenValidator, error) {
	validators := make([]TokenValidator, 0)
	for _, config := range configs {
		config := config
		validator, err := NewValidatorFromConfig(&config, opts...)
		if err != nil {
			return nil, fmt.Errorf("error creating validator from config: %w", err)
		}
		validators = append(validators, validator)
	}
	return validators, nil
}
