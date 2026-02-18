package oidc

import (
	"github.com/dioad/util"

	"github.com/dioad/net/tls"
)

type EndpointConfig struct {
	Type          string `json:"type,omitempty" mapstructure:"type,omitempty"`
	URL           string `json:"url" mapstructure:"url"`
	KeycloakRealm string `json:"keycloak-realm,omitempty" mapstructure:"keycloak-realm,omitempty"`
}

type ClientConfig struct {
	// Provider     EndpointConfig    `json:"provider"` // e.g. "github", "keycloak"
	EndpointConfig `mapstructure:",squash"`
	ClientID       string            `json:"client-id" mapstructure:"client-id"`
	ClientSecret   util.MaskedString `json:"client-secret" mapstructure:"client-secret,omitempty"`

	Audience string `json:"audience,omitempty" mapstructure:"audience,omitempty"`

	// do these belong somewhere else?
	TokenFile string `json:"token-file,omitempty" mapstructure:"token-file,omitempty"`

	TLSConfig tls.ClientConfig `json:"tls-client-config" mapstructure:"tls-client-config,omitempty"`
}

type ValidatorConfig struct {
	EndpointConfig     `mapstructure:",squash"`
	Audiences          []string       `json:"audiences" mapstructure:"audiences"`
	Issuer             string         `json:"issuer" mapstructure:"issuer"`
	CacheTTL           int            `json:"cache_ttl_seconds" mapsstructure:"cache_ttl_seconds"`
	SignatureAlgorithm string         `json:"signature_algorithm" mapstructure:"signature_algorithm"`
	AllowedClockSkew   int            `json:"allowed_clock_skew_seconds" mapstructure:"allowed_clock_skew_seconds"`
	Debug              bool           `json:"debug" mapstructure:"debug"`
	ClaimPredicate     map[string]any `json:"claim_predicates" mapstructure:"claim_predicates"`
}

type TrustConfig struct {
	Verifiers []ValidatorConfig `json:"validators" mapstructure:"validators"`
}
