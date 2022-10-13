package oidc

import "github.com/dioad/net/http"

const (
	SessionCookieName        = "dioad_session"
	PreAuthRefererCookieName = "auth_referer"
)

type ProviderConfig struct {
	ClientID     string `mapstructure:"client-id"`
	ClientSecret string `mapstructure:"client-secret"`
	Callback     string `mapstructure:"callback"`
}

type ProviderMap map[string]ProviderConfig

type Config struct {
	ProviderMap  ProviderMap       `mapstructure:"providers"`
	CookieConfig http.CookieConfig `mapstructure:"cookies"`
}
