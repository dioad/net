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

// ValidatorDebugger wraps a TokenValidator with debug logging capabilities.
type ValidatorDebugger struct {
	logger          zerolog.Logger
	parentValidator TokenValidator
}

// PredicateValidator wraps a TokenValidator and applies additional claim predicate validation.
type PredicateValidator struct {
	parentValidator TokenValidator
	predicate       ClaimPredicate
}

// TokenValidator defines the interface for validating tokens.
type TokenValidator interface {
	ValidateToken(ctx context.Context, tokenString string) (interface{}, error)
	String() string
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

// ValidatorDebugOpts is a functional option for configuring a ValidatorDebugger.
type ValidatorDebugOpts func(*ValidatorDebugger)

func NewPredicateValidator(validator TokenValidator, predicate ClaimPredicate) *PredicateValidator {
	return &PredicateValidator{
		parentValidator: validator,
		predicate:       predicate,
	}
}

func (v *PredicateValidator) ValidateToken(ctx context.Context, tokenString string) (interface{}, error) {
	claims, err := v.parentValidator.ValidateToken(ctx, tokenString)
	if err != nil {
		return nil, fmt.Errorf("error validating token: %w", err)
	}

	claimsMap, err := ExtractClaimsMap(tokenString)
	if err != nil {
		return nil, fmt.Errorf("error extracting claims map: %w", err)
	}

	if !v.predicate.Validate(claimsMap) {
		return nil, fmt.Errorf("predicate validation failed")
	}

	return claims, nil
}

func (v *PredicateValidator) String() string {
	return fmt.Sprintf("PredicateValidator(validator:%v, predicate:%v)", v.parentValidator, v.predicate)
}

func decodeTokenData(accessToken string) (interface{}, error) {
	// Decode Access Token and extract expiry andPredicate any other details necessary from the token
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

	if expiry, ok := tokenData["exp"].(float64); ok {
		tokenData["exp_datetime"] = time.Unix(int64(expiry), 0)
	}

	if issuedAt, ok := tokenData["iat"].(float64); ok {
		tokenData["iat_datetime"] = time.Unix(int64(issuedAt), 0)
	}

	if notBefore, ok := tokenData["nbf"].(float64); ok {
		tokenData["nbf_datetime"] = time.Unix(int64(notBefore), 0)
	}

	return tokenData, nil
}

func NewValidatorDebugger(validator TokenValidator, opts ...ValidatorDebugOpts) *ValidatorDebugger {
	v := &ValidatorDebugger{
		parentValidator: validator,
	}

	for _, o := range opts {
		o(v)
	}

	return v
}

func (v *ValidatorDebugger) ValidateToken(ctx context.Context, tokenString string) (interface{}, error) {
	tokenDetails, err := decodeTokenData(tokenString)
	if err != nil {
		return nil, fmt.Errorf("error decoding token data: %w", err)
	}

	v.logger.Debug().
		Stringer("validator", v.parentValidator).
		Interface("DecodedToken", tokenDetails).
		Msg("decoded token")
	claims, err := v.parentValidator.ValidateToken(ctx, tokenString)
	if err != nil {
		v.logger.Error().Err(err).Msg("error validating token")
	}
	return claims, err
}

func (v *ValidatorDebugger) String() string {
	return fmt.Sprintf("ValidatorDebugger(%v)", v.parentValidator)
}

// MultiValidator attempts to validate tokens using multiple validators in sequence.
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

	return nil, fmt.Errorf("%w: %w, %v", ErrTokenValidation, err, strings.Join(errs, ", "))
}

func (v *MultiValidator) String() string {
	validators := make([]string, len(v.validators))
	for i, vtor := range v.validators {
		validators[i] = vtor.String()
	}
	return fmt.Sprintf("MultiValidator(%v)", strings.Join(validators, ", "))
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

type jwtValidator struct {
	jwtValidator *jwtvalidator.Validator

	endpoint           Endpoint
	cacheTTL           time.Duration
	signatureAlgorithm jwtvalidator.SignatureAlgorithm
	issuerURL          string
	allowedClockSkew   time.Duration
}

func (v *jwtValidator) ValidateToken(ctx context.Context, tokenString string) (interface{}, error) {
	return v.jwtValidator.ValidateToken(ctx, tokenString)
}

func (v *jwtValidator) String() string {
	return fmt.Sprintf("jwtValidator(endpoint:%v, issuer:%v, signatureAlgorithm:%v)", v.endpoint.URL().String(), v.issuerURL, v.signatureAlgorithm)
}

func newJWTValidator(config *ValidatorConfig, opts ...jwtvalidator.Option) (*jwtValidator, error) {
	v := &jwtValidator{}

	var err error

	v.endpoint, err = NewEndpointFromConfig(&config.EndpointConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating endpoint from config: %w", err)
	}

	v.cacheTTL = 5 * time.Minute
	if config.CacheTTL != 0 {
		v.cacheTTL = time.Duration(config.CacheTTL) * time.Second
	}

	v.signatureAlgorithm = jwtvalidator.RS256
	if config.SignatureAlgorithm != "" {
		v.signatureAlgorithm = jwtvalidator.SignatureAlgorithm(config.SignatureAlgorithm)
	}

	v.issuerURL = config.URL
	if config.Issuer != "" {
		v.issuerURL = config.Issuer
	}

	v.allowedClockSkew = time.Minute
	if config.AllowedClockSkew != 0 {
		v.allowedClockSkew = time.Duration(config.AllowedClockSkew) * time.Second
	}

	opts = append([]jwtvalidator.Option{
		jwtvalidator.WithAllowedClockSkew(v.allowedClockSkew),
	}, opts...)

	provider := jwks.NewCachingProvider(v.endpoint.URL(), v.cacheTTL)

	// var validator TokenValidator
	v.jwtValidator, err = jwtvalidator.New(
		provider.KeyFunc,
		v.signatureAlgorithm,
		v.issuerURL,
		config.Audiences,
		opts...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to configure validator for %v: %w", config.URL, err)
	}

	return v, nil
}

func NewValidatorFromConfig(config *ValidatorConfig, opts ...jwtvalidator.Option) (TokenValidator, error) {
	v, err := newJWTValidator(config, opts...)
	if err != nil {
		return nil, err
	}

	var validator TokenValidator
	validator = v

	if config.ClaimPredicate != nil {
		predicate := ParseClaimPredicates(config.ClaimPredicate)
		validator = NewPredicateValidator(validator, predicate)
	}

	if config.Debug {
		validator = NewValidatorDebugger(validator,
			WithLabel("issuer", v.issuerURL),
			WithLabel("type", config.Type),
			WithLabel("audiences", strings.Join(config.Audiences, ",")),
			WithLabel("signatureAlgorithm", config.SignatureAlgorithm),
			WithLabel("allowedClockSkew", v.allowedClockSkew.String()),
			WithLabel("cacheTTL", v.cacheTTL.String()),
		)
	}

	return validator, nil
}

func NewValidatorsFromConfig(configs []ValidatorConfig, opts ...jwtvalidator.Option) ([]TokenValidator, error) {
	validators := make([]TokenValidator, 0)
	for _, config := range configs {
		validator, err := NewValidatorFromConfig(&config, opts...)
		if err != nil {
			return nil, fmt.Errorf("error creating parentValidator from config: %w", err)
		}
		validators = append(validators, validator)
	}
	return validators, nil
}
