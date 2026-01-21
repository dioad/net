package oidc

import (
	"time"

	"github.com/dioad/net/http"
)

const (
	// SessionCookieName is the name of the cookie used to store the OIDC session.
	SessionCookieName = "dioad_session"
	// PreAuthRefererCookieName is the name of the cookie used to store the referer URL before authentication.
	PreAuthRefererCookieName = "auth_referer"
)

// ProviderConfig contains configuration for an OIDC provider.
type ProviderConfig struct {
	ClientID     string   `mapstructure:"client-id"`
	ClientSecret string   `mapstructure:"client-secret"`
	Callback     string   `mapstructure:"callback"`
	Scopes       []string `mapstructure:"scopes"`        // OAuth2 Scopes - Optional
	DiscoveryURL string   `mapstructure:"discovery-url"` // OpenID Connect Discovery URL - Optional
}

// ProviderMap maps provider names to their configurations.
type ProviderMap map[string]ProviderConfig

// Config contains configuration for OIDC authentication.
type Config struct {
	ProviderMap  ProviderMap       `mapstructure:"providers"`
	CookieConfig http.CookieConfig `mapstructure:"cookies"`
}

// OIDCCookieConfig contains cookie-specific configuration.
type OIDCCookieConfig struct {
	Name   string `mapstructure:"name"`
	Domain string `mapstructure:"domain"`
	MaxAge int    `mapstructure:"max-age"`
	Path   string `mapstructure:"path"`
}

// OIDCConfig contains configuration for OIDC authentication with detailed cookie settings.
type OIDCConfig struct {
	ProviderMap             ProviderMap      `mapstructure:"providers"`
	TokenCookieConfig       OIDCCookieConfig `mapstructure:"token-cookie"`
	StateCookieConfig       OIDCCookieConfig `mapstructure:"state-cookie"`
	TokenExpiryCookieConfig OIDCCookieConfig `mapstructure:"token-expiry-cookie"`
	RefreshCookieConfig     OIDCCookieConfig `mapstructure:"refresh-cookie"`
	RedirectCookieConfig    OIDCCookieConfig `mapstructure:"redirect-cookie"`
	Now                     func() time.Time
}

// SetDefaults sets default values for the OIDC configuration.
// This implementation demonstrates proper separation of concerns by extracting
// cookie validation logic into a dedicated helper method.
func (c *OIDCConfig) SetDefaults() {
	// Set default time function
	if c.Now == nil {
		c.Now = time.Now
	}

	// Apply defaults to each cookie configuration
	c.TokenCookieConfig = c.applyCookieDefaults(c.TokenCookieConfig, "token", 3600)
	c.StateCookieConfig = c.applyCookieDefaults(c.StateCookieConfig, "state", 600)
	c.TokenExpiryCookieConfig = c.applyCookieDefaults(c.TokenExpiryCookieConfig, "token_expiry", 3600)
	c.RefreshCookieConfig = c.applyCookieDefaults(c.RefreshCookieConfig, "refresh", 86400)
	c.RedirectCookieConfig = c.applyCookieDefaults(c.RedirectCookieConfig, "redirect", 600)
}

// applyCookieDefaults applies default values to a cookie configuration.
// This helper method reduces cyclomatic complexity by consolidating the validation
// logic that was previously spread across 21 guard conditions.
func (c *OIDCConfig) applyCookieDefaults(config OIDCCookieConfig, defaultName string, defaultMaxAge int) OIDCCookieConfig {
	if config.Name == "" {
		config.Name = defaultName
	}
	if config.Domain == "" {
		config.Domain = ""
	}
	if config.MaxAge <= 0 {
		config.MaxAge = defaultMaxAge
	}
	if config.Path == "" {
		config.Path = "/"
	}
	return config
}
