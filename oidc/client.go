package oidc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/auth0/go-jwt-middleware/v2/jwks"
	jwtvalidator "github.com/auth0/go-jwt-middleware/v2/validator"
	"github.com/markbates/goth"
	"golang.org/x/oauth2"

	"github.com/dioad/net/oidc/flyio"
)

var (
	ErrInvalidToken    = errors.New("invalid token")
	ErrTokenValidation = errors.New("token validation failed")
	ErrInvalidClaims   = errors.New("invalid claims format")
)

type IntrospectionResponse struct {
	ExpiresAt                           int      `json:"exp"`
	IssuedAt                            int      `json:"iat"`
	AuthTime                            int      `json:"auth_time"`
	ID                                  string   `json:"jti"`
	Issuer                              string   `json:"iss"`
	Audience                            string   `json:"aud"`
	Subject                             string   `json:"sub"`
	Type                                string   `json:"typ"`
	AuthorizedParty                     string   `json:"azp"`
	SessionID                           string   `json:"sid"`
	AuthenticationContextClassReference string   `json:"acr"`
	AllowedOrigins                      []string `json:"allowed-origins"`
	RealmAccess                         struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
	ResourceAccess struct {
		Account struct {
			Roles []string `json:"roles"`
		} `json:"account"`
	} `json:"resource_access"`
	Scope             string   `json:"scope"`
	UserPrincipalName string   `json:"upn"`
	EmailVerified     bool     `json:"email_verified"`
	Name              string   `json:"name"`
	Groups            []string `json:"groups"`
	PreferredUsername string   `json:"preferred_username"`
	GivenName         string   `json:"given_name"`
	FamilyName        string   `json:"family_name"`
	Email             string   `json:"email"`
	ClientId          string   `json:"client_id"`
	Username          string   `json:"username"`
	TokenType         string   `json:"token_type"`
	Active            bool     `json:"active"`
	Website           string   `json:"website"`
	Organisation      []string `json:"org"`
	flyio.Claims      `json:",squash"`
}

func (c *IntrospectionResponse) Validate(ctx context.Context) error {
	return nil
}

type ClientOpt func(c *Client)

func WithClientID(clientID string) ClientOpt {
	return func(c *Client) {
		c.clientID = clientID
	}
}

func WithKeyCacheTTL(ttl time.Duration) ClientOpt {
	return func(c *Client) {
		c.keyCacheTTL = ttl
	}
}

func WithValidatingSignatureAlgorithm(algorithm jwtvalidator.SignatureAlgorithm) ClientOpt {
	return func(c *Client) {
		c.validatingSignatureAlgorithm = algorithm
	}
}

func WithClientIDAndSecret(clientID, clientSecret string) ClientOpt {
	return func(c *Client) {
		c.clientID = clientID
		c.clientSecret = clientSecret
	}
}

type oAuth2ConfigOpt func(c *oauth2.Config)

func withScopes(scopes ...string) oAuth2ConfigOpt {
	return func(c *oauth2.Config) {
		c.Scopes = scopes
	}
}

func withClientSecret(clientSecret string) oAuth2ConfigOpt {
	return func(c *oauth2.Config) {
		c.ClientSecret = clientSecret
	}
}

func withRedirectURL(redirectURL string) oAuth2ConfigOpt {
	return func(c *oauth2.Config) {
		c.RedirectURL = redirectURL
	}
}

func WithDeviceCodeUI(ui DeviceCodeUI) ClientOpt {
	return func(c *Client) {
		c.deviceUI = ui
	}
}

type Client struct {
	endpoint                     Endpoint
	jwksProvider                 *jwks.CachingProvider
	keyCacheTTL                  time.Duration
	clientID                     string
	clientSecret                 string
	validatingSignatureAlgorithm jwtvalidator.SignatureAlgorithm
	deviceUI                     DeviceCodeUI
}

// NewHTTPClientFromConfig

func NewClientFromConfig(config *ClientConfig) (*Client, error) {
	endpoint, err := NewEndpointFromConfig(&config.EndpointConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create new oidc endpoint: %w", err)
	}
	return NewClient(endpoint, WithClientIDAndSecret(config.ClientID, config.ClientSecret.MaskedString())), nil
}

func NewClient(endpoint Endpoint, opts ...ClientOpt) *Client {
	client := &Client{
		endpoint:    endpoint,
		keyCacheTTL: 5 * time.Minute,
	}

	for _, opt := range opts {
		opt(client)
	}

	client.jwksProvider = jwks.NewCachingProvider(endpoint.URL(), client.keyCacheTTL)

	return client
}

func (c *Client) Endpoint() Endpoint {
	return c.endpoint
}

func (c *Client) GothProvider(callbackURL *url.URL, scopes ...string) (goth.Provider, error) {
	ge, ok := c.endpoint.(GothEndpoint)
	if !ok {
		return nil, fmt.Errorf("endpoint does not support goth provider")
	}

	return ge.GothProvider(c.clientID, c.clientSecret, callbackURL, scopes...)
}

// oAuth2Config returns an OAuth2 configuration for the OIDC client
func (c *Client) oAuth2Config(opts ...oAuth2ConfigOpt) (*oauth2.Config, error) {
	oauth2Endpoint, err := c.endpoint.OAuth2Endpoint()
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth2 endpoint: %w", err)
	}

	config := &oauth2.Config{
		ClientID:     c.clientID,
		ClientSecret: c.clientSecret,
		Endpoint:     oauth2Endpoint,
	}

	for _, opt := range opts {
		opt(config)
	}

	return config, nil
}

// ValidateToken VerifyToke verifies the token and returns the claims
// It fetches the verification keys from the OIDC server
// and uses them to verify the token
func (c *Client) ValidateToken(ctx context.Context, token string, audiences []string) (*jwtvalidator.ValidatedClaims, error) {
	jwtValidator, err := jwtvalidator.New(
		c.jwksProvider.KeyFunc,
		c.validatingSignatureAlgorithm,
		c.endpoint.URL().String(),
		audiences,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to configure validator for %v: %w", c.endpoint.URL(), err)
	}

	validatedClaims, err := jwtValidator.ValidateToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("error validating token: %w", err)
	}

	return validatedClaims.(*jwtvalidator.ValidatedClaims), nil
}

type RequestOpt func(url.Values)

func WithAudience(audience string) RequestOpt {
	return func(v url.Values) {
		if audience != "" {
			v.Set("audience", audience)
		}
	}
}

// ClientCredentialsToken gets a token using the client_credentials grant
// It sends the client_id and client_secret to the token endpoint
// and gets a token in response
func (c *Client) ClientCredentialsToken(ctx context.Context, opts ...RequestOpt) (*oauth2.Token, error) {
	discoveredConfiguration, err := c.endpoint.DiscoveredConfiguration()
	if err != nil {
		return nil, err
	}
	tokenURL := discoveredConfiguration.TokenEndpoint
	if tokenURL == "" {
		return nil, fmt.Errorf("token endpoint not available")
	}

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", c.clientID)
	data.Set("client_secret", c.clientSecret)

	for _, opt := range opts {
		opt(data)
	}

	tokenResponse, err := doPost[oauth2.Token](ctx, tokenURL, data)
	if err != nil {
		return nil, err
	}

	return tokenResponse, nil
}

// IntrospectToken introspects the token
// It sends the token to the introspection endpoint
// and gets the response
func (c *Client) IntrospectToken(ctx context.Context, token string) (*IntrospectionResponse, error) {
	discoveredConfiguration, err := c.endpoint.DiscoveredConfiguration()
	if err != nil {
		return nil, err
	}
	introspectionURL := discoveredConfiguration.IntrospectionEndpoint
	if introspectionURL == "" {
		return nil, fmt.Errorf("introspection endpoint not available")
	}

	data := url.Values{}
	data.Set("token", token)

	introspectionResponse, err := doPostWithBasicAuth[IntrospectionResponse](ctx, introspectionURL, data, c.clientID, c.clientSecret)
	if err != nil {
		return nil, err
	}

	return introspectionResponse, nil
}

func (c *Client) DeviceToken(ctx context.Context, scopes ...string) (*oauth2.Token, error) {
	config, err := c.oAuth2Config(withScopes(scopes...))
	if err != nil {
		return nil, fmt.Errorf("error getting OAuth2 config: %w", err)
	}

	deviceCode, err := config.DeviceAuth(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting device code: %w", err)
	}

	deviceCodeUI := DeviceCodeUIConsoleText
	if c.deviceUI != nil {
		deviceCodeUI = c.deviceUI
	}

	err = deviceCodeUI(deviceCode)
	if err != nil {
		return nil, fmt.Errorf("error displaying device code: %w", err)
	}

	token, err := config.DeviceAccessToken(ctx, deviceCode)
	if err != nil {
		return nil, fmt.Errorf("error exchanging device code for access token: %w", err)
	}

	return token, err
}

func (c *Client) HTTPClient(ctx context.Context, t *oauth2.Token) (*http.Client, error) {
	oauth2Config, err := c.oAuth2Config()
	if err != nil {
		return nil, fmt.Errorf("error getting OAuth2 config: %w", err)
	}
	return oauth2Config.Client(ctx, t), nil
}

func (c *Client) TokenSource(t *oauth2.Token) (oauth2.TokenSource, error) {
	oauth2Config, err := c.oAuth2Config()
	if err != nil {
		return nil, fmt.Errorf("error getting OAuth2 config: %w", err)
	}
	return oauth2Config.TokenSource(context.Background(), t), nil
}

func ExtractClaims[T jwtvalidator.CustomClaims](claims interface{}) (jwtvalidator.RegisteredClaims, T, error) {
	var zeroCustomClaims T
	var zeroRegisteredClaims jwtvalidator.RegisteredClaims

	validatedClaims, ok := claims.(*jwtvalidator.ValidatedClaims)
	if !ok {
		return zeroRegisteredClaims, zeroCustomClaims,
			fmt.Errorf("error extracting claims")
	}

	if validatedClaims.CustomClaims == nil {
		return validatedClaims.RegisteredClaims, zeroCustomClaims, nil
	}

	customClaims, ok := validatedClaims.CustomClaims.(T)
	if !ok {
		return validatedClaims.RegisteredClaims, zeroCustomClaims,
			fmt.Errorf("%w: expected custom claims type %T, got %T",
				ErrInvalidClaims, zeroCustomClaims, validatedClaims.CustomClaims)
	}

	return validatedClaims.RegisteredClaims, customClaims, nil
}
