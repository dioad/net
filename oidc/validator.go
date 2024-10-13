package oidc

import (
	"context"
	"fmt"
	"time"

	"github.com/auth0/go-jwt-middleware/v2/jwks"
	jwtvalidator "github.com/auth0/go-jwt-middleware/v2/validator"
)

type MultiValidator struct {
	validators []*jwtvalidator.Validator
}

func (v *MultiValidator) ValidateToken(ctx context.Context, tokenString string) (interface{}, error) {
	var err error
	var claims interface{}
	for _, vtor := range v.validators {
		claims, err = vtor.ValidateToken(ctx, tokenString)
		if err == nil {
			return claims, nil
		}
	}

	return nil, fmt.Errorf("token validation failed: %w", err)
}

func NewMultiValidator(validators ...*jwtvalidator.Validator) *MultiValidator {
	return &MultiValidator{validators: validators}
}

func NewMultiValidatorFromConfig(configs []VerifierConfig, opts ...jwtvalidator.Option) (*MultiValidator, error) {
	validators, err := NewValidatorsFromConfig(configs, opts...)
	if err != nil {
		return nil, fmt.Errorf("error creating validators from config: %w", err)
	}
	return NewMultiValidator(validators...), nil
}

func NewValidatorFromConfig(config *VerifierConfig, opts ...jwtvalidator.Option) (*jwtvalidator.Validator, error) {
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

	jwtValidator, err := jwtvalidator.New(
		provider.KeyFunc,
		signatureAlgorithm,
		issuerURL,
		config.Audiences,
		opts...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to configure validator for %v: %w", config.URL, err)
	}

	return jwtValidator, nil
}

func NewValidatorsFromConfig(configs []VerifierConfig, opts ...jwtvalidator.Option) ([]*jwtvalidator.Validator, error) {
	validators := make([]*jwtvalidator.Validator, 0)
	for _, config := range configs {
		validator, err := NewValidatorFromConfig(&config, opts...)
		if err != nil {
			return nil, fmt.Errorf("error creating validator from config: %w", err)
		}
		validators = append(validators, validator)
	}
	return validators, nil
}
