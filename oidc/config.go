package oidc

import "github.com/dioad/util"

type EndpointConfig struct {
	Type          string `json:"type,omitempty"`
	URL           string `json:"url"`
	KeycloakRealm string `json:"keycloak_realm,omitempty"`
}

type ClientConfig struct {
	Provider     EndpointConfig    `json:"provider"` // e.g. "github", "keycloak"
	ClientID     string            `json:"client-id"`
	ClientSecret util.MaskedString `json:"client-secret,omitempty"`

	Audience string `json:"audience,omitempty"`

	// do these belong somewhere else?
	TokenFile string `json:"token-file"`
}

type VerifierConfig struct {
	EndpointConfig
	Audiences          []string `json:"audiences"`
	Issuer             string   `json:"issuer"`
	CacheTTL           int      `json:"cache_ttl_seconds"`
	SignatureAlgorithm string   `json:"signature_algorithm"`
	AllowedClockSkew   int      `json:"allowed_clock_skew_seconds"`
}

type TrustConfig struct {
	Verifiers []VerifierConfig `json:"verifiers"`
}
