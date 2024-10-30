package oidc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/openidConnect"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"
)

/*
Several extensions are documented and published for the openid-configuration endpoint. These extensions enhance the functionality and security of OpenID Connect and OAuth 2.0. Here are some notable extensions:
OAuth 2.0 Mutual-TLS Client Authentication and Certificate-Bound Access Tokens (RFC 8705):
mtls_endpoint_aliases: Provides alternative endpoints secured using mutual TLS (mTLS).
OAuth 2.0 Device Authorization Grant (RFC 8628):
device_authorization_endpoint: Specifies the endpoint for the device authorization grant.
OAuth 2.0 Authorization Server Metadata (RFC 8414):
authorization_response_iss_parameter_supported: Indicates if the iss parameter is supported in the authorization response.
require_pushed_authorization_requests: Indicates if pushed authorization requests are required.
pushed_authorization_request_endpoint: Specifies the endpoint for pushed authorization requests.
OAuth 2.0 Incremental Authorization:
incremental_authz_supported: Indicates if incremental authorization is supported.
OAuth 2.0 Rich Authorization Requests (RAR):
authorization_details_types_supported: Lists the types of authorization details supported.
Financial-grade API (FAPI):
tls_client_certificate_bound_access_tokens: Indicates if access tokens are bound to client certificates.
These extensions are defined in their respective RFCs and specifications, providing additional capabilities and security features for OpenID Connect and OAuth 2.0 implementations.
*/

type OpenIDConfiguration struct {
	Issuer                                                    string   `json:"issuer"`
	AuthorizationEndpoint                                     string   `json:"authorization_endpoint"`
	TokenEndpoint                                             string   `json:"token_endpoint"`
	UserinfoEndpoint                                          string   `json:"userinfo_endpoint"`
	JWKSURI                                                   string   `json:"jwks_uri"`
	RegistrationEndpoint                                      string   `json:"registration_endpoint"`
	ScopesSupported                                           []string `json:"scopes_supported"`
	ResponseTypesSupported                                    []string `json:"response_types_supported"`
	GrantTypesSupported                                       []string `json:"grant_types_supported"`
	SubjectTypesSupported                                     []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported                          []string `json:"id_token_signing_alg_values_supported"`
	TokenEndpointAuthMethodsSupported                         []string `json:"token_endpoint_auth_methods_supported"`
	ClaimsSupported                                           []string `json:"claims_supported"`
	CodeChallengeMethodsSupported                             []string `json:"code_challenge_methods_supported"`
	IntrospectionEndpoint                                     string   `json:"introspection_endpoint"`
	EndSessionEndpoint                                        string   `json:"end_session_endpoint"`
	FrontchannelLogoutSessionSupported                        bool     `json:"frontchannel_logout_session_supported"`
	FrontchannelLogoutSupported                               bool     `json:"frontchannel_logout_supported"`
	CheckSessionIframe                                        string   `json:"check_session_iframe"`
	AcrValuesSupported                                        []string `json:"acr_values_supported"`
	IDTokenEncryptionAlgValuesSupported                       []string `json:"id_token_encryption_alg_values_supported"`
	IDTokenEncryptionEncValuesSupported                       []string `json:"id_token_encryption_enc_values_supported"`
	UserinfoSigningAlgValuesSupported                         []string `json:"userinfo_signing_alg_values_supported"`
	UserinfoEncryptionAlgValuesSupported                      []string `json:"userinfo_encryption_alg_values_supported"`
	UserinfoEncryptionEncValuesSupported                      []string `json:"userinfo_encryption_enc_values_supported"`
	RequestObjectSigningAlgValuesSupported                    []string `json:"request_object_signing_alg_values_supported"`
	RequestObjectEncryptionAlgValuesSupported                 []string `json:"request_object_encryption_alg_values_supported"`
	RequestObjectEncryptionEncValuesSupported                 []string `json:"request_object_encryption_enc_values_supported"`
	ResponseModesSupported                                    []string `json:"response_modes_supported"`
	TokenEndpointAuthSigningAlgValuesSupported                []string `json:"token_endpoint_auth_signing_alg_values_supported"`
	IntrospectionEndpointAuthMethodsSupported                 []string `json:"introspection_endpoint_auth_methods_supported"`
	IntrospectionEndpointAuthSigningAlgValuesSupported        []string `json:"introspection_endpoint_auth_signing_alg_values_supported"`
	AuthorizationSigningAlgValuesSupported                    []string `json:"authorization_signing_alg_values_supported"`
	AuthorizationEncryptionAlgValuesSupported                 []string `json:"authorization_encryption_alg_values_supported"`
	AuthorizationEncryptionEncValuesSupported                 []string `json:"authorization_encryption_enc_values_supported"`
	ClaimTypesSupported                                       []string `json:"claim_types_supported"`
	ClaimsParameterSupported                                  bool     `json:"claims_parameter_supported"`
	RequestParameterSupported                                 bool     `json:"request_parameter_supported"`
	RequestURIParameterSupported                              bool     `json:"request_uri_parameter_supported"`
	RequireRequestURIRegistration                             bool     `json:"require_request_uri_registration"`
	TLSClientCertificateBoundAccessTokens                     bool     `json:"tls_client_certificate_bound_access_tokens"`
	RevocationEndpoint                                        string   `json:"revocation_endpoint"`
	RevocationEndpointAuthMethodsSupported                    []string `json:"revocation_endpoint_auth_methods_supported"`
	RevocationEndpointAuthSigningAlgValuesSupported           []string `json:"revocation_endpoint_auth_signing_alg_values_supported"`
	BackchannelLogoutSupported                                bool     `json:"backchannel_logout_supported"`
	BackchannelLogoutSessionSupported                         bool     `json:"backchannel_logout_session_supported"`
	DeviceAuthorizationEndpoint                               string   `json:"device_authorization_endpoint"`
	BackchannelTokenDeliveryModesSupported                    []string `json:"backchannel_token_delivery_modes_supported"`
	BackchannelAuthenticationEndpoint                         string   `json:"backchannel_authentication_endpoint"`
	BackchannelAuthenticationRequestSigningAlgValuesSupported []string `json:"backchannel_authentication_request_signing_alg_values_supported"`
	RequirePushedAuthorizationRequests                        bool     `json:"require_pushed_authorization_requests"`
	PushedAuthorizationRequestEndpoint                        string   `json:"pushed_authorization_request_endpoint"`
	MTLSEndpointAliases                                       struct {
		TokenEndpoint                      string `json:"token_endpoint"`
		RevocationEndpoint                 string `json:"revocation_endpoint"`
		IntrospectionEndpoint              string `json:"introspection_endpoint"`
		DeviceAuthorizationEndpoint        string `json:"device_authorization_endpoint"`
		RegistrationEndpoint               string `json:"registration_endpoint"`
		UserinfoEndpoint                   string `json:"userinfo_endpoint"`
		PushedAuthorizationRequestEndpoint string `json:"pushed_authorization_request_endpoint"`
		BackchannelAuthenticationEndpoint  string `json:"backchannel_authentication_endpoint"`
	} `json:"mtls_endpoint_aliases"`
	AuthorizationResponseIssParameterSupported bool `json:"authorization_response_iss_parameter_supported"`
}

type Endpoint interface {
	URL() *url.URL
	DiscoveryEndpoint() (*url.URL, error)
	DiscoveredConfiguration() (*OpenIDConfiguration, error)
	OAuth2Endpoint() (oauth2.Endpoint, error)
}

type GothEndpoint interface {
	GothProvider(clientID, clientSecret string, callbackURL *url.URL, scopes ...string) (goth.Provider, error)
}

type oidcEndpoint struct {
	url *url.URL
}

func (e *oidcEndpoint) URL() *url.URL {
	return e.url
}

func (e *oidcEndpoint) DiscoveryEndpoint() (*url.URL, error) {
	return e.url.JoinPath(".well-known", "openid-configuration"), nil
}

func (e *oidcEndpoint) DiscoveredConfiguration() (*OpenIDConfiguration, error) {
	discoveryEndpoint, _ := e.DiscoveryEndpoint()
	response, err := http.Get(discoveryEndpoint.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get discovery URL %v: %w", discoveryEndpoint, err)
	}
	discoveredConfiguration := OpenIDConfiguration{}

	jsonDecoder := json.NewDecoder(response.Body)
	err = jsonDecoder.Decode(&discoveredConfiguration)
	return &discoveredConfiguration, err
}

func (e *oidcEndpoint) OAuth2Endpoint() (oauth2.Endpoint, error) {
	discoveredConfiguration, err := e.DiscoveredConfiguration()
	if err != nil {
		return oauth2.Endpoint{}, err
	}

	return oauth2.Endpoint{
		AuthURL:       discoveredConfiguration.AuthorizationEndpoint,
		TokenURL:      discoveredConfiguration.TokenEndpoint,
		DeviceAuthURL: discoveredConfiguration.DeviceAuthorizationEndpoint,
	}, nil
}

func (e *oidcEndpoint) GothProvider(clientID, clientSecret string, callbackURL *url.URL, scopes ...string) (goth.Provider, error) {
	discoveryEndpoint, err := e.DiscoveryEndpoint()
	if err != nil {
		return nil, err
	}

	return openidConnect.New(
		clientID,
		clientSecret,
		callbackURL.String(),
		discoveryEndpoint.String(),
		scopes...)
}

func NewEndpoint(baseURL string) (Endpoint, error) {
	u, _ := url.Parse(baseURL)

	return &oidcEndpoint{
		url: u,
	}, nil
}

func NewEndpointFromConfig(config *EndpointConfig) (Endpoint, error) {
	if config.Type == "github" {
		githubURL := config.URL
		if githubURL == "" {
			githubURL = "https://github.com"
		}
		return NewGitHubEndpoint(githubURL)
	}

	if config.Type == "keycloak" {
		return NewKeycloakRealmEndpoint(config.URL, config.KeycloakRealm)
	}

	if config.Type == "oidc" {
		return NewEndpoint(config.URL)
	}

	return nil, fmt.Errorf("config type %s not supported", config.Type)
}

type KeycloakEndpoint struct {
	url *url.URL
}

func (e *KeycloakEndpoint) RealmEndpoint(realm string) Endpoint {
	return &oidcEndpoint{url: e.url.JoinPath("realms", realm)}
}

func NewKeycloakEndpoint(baseURLStr string) (*KeycloakEndpoint, error) {
	baseURL, err := url.Parse(baseURLStr)
	if err != nil {
		return nil, err
	}
	return &KeycloakEndpoint{url: baseURL}, nil
}

func NewKeycloakRealmEndpoint(baseURLStr, realm string) (Endpoint, error) {
	keycloakEndpoint, err := NewKeycloakEndpoint(baseURLStr)
	if err != nil {
		return nil, fmt.Errorf("error creating keycloak endpoint: %w", err)
	}

	return keycloakEndpoint.RealmEndpoint(realm), nil
}

type GitHubEndpoint struct {
	url *url.URL
}

func (e *GitHubEndpoint) URL() *url.URL {
	return e.url
}

func (e *GitHubEndpoint) DiscoveryEndpoint() (*url.URL, error) {
	return nil, fmt.Errorf("GitHub does not support OpenID Connect discovery")
}

func (e *GitHubEndpoint) DiscoveredConfiguration() (*OpenIDConfiguration, error) {
	return &OpenIDConfiguration{
		AuthorizationEndpoint:       e.url.JoinPath("/login/oauth/authorize").String(),
		TokenEndpoint:               e.url.JoinPath("/login/oauth/access_token").String(),
		DeviceAuthorizationEndpoint: e.url.JoinPath("/login/device/code").String(),
	}, nil
}

func (e *GitHubEndpoint) OAuth2Endpoint() (oauth2.Endpoint, error) {
	return endpoints.GitHub, nil
}

func (e *GitHubEndpoint) GothProvider(clientID, clientSecret string, callbackURL *url.URL, scopes ...string) (goth.Provider, error) {
	return github.New(clientID, clientSecret, callbackURL.String(), scopes...), nil
}

func NewGitHubEndpoint(baseURL string) (Endpoint, error) {
	u, _ := url.Parse(baseURL)

	return &GitHubEndpoint{url: u}, nil
}
