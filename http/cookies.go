package http

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/pkg/errors"
)

// CookieConfig describes the configuration for HTTP cookies.
type CookieConfig struct {
	Base64AuthenticationKey string `mapstructure:"base64-authentication-key"`
	Base64EncryptionKey     string `mapstructure:"base64-encryption-key"`
	MaxAge                  int    `mapstructure:"max-age"`
	Domain                  string `mapstructure:"domain"`
}

// NewPersistentCookieStore creates a persistent cookie store from the provided configuration.
func NewPersistentCookieStore(config CookieConfig) (*sessions.CookieStore, error) {
	store, err := NewSessionCookieStore(config)
	if err != nil {
		return nil, err
	}
	store.MaxAge(config.MaxAge)

	return store, nil
}

// NewSessionCookieStore creates a session cookie store from the provided configuration.
func NewSessionCookieStore(config CookieConfig) (*sessions.CookieStore, error) {
	authKey, err := base64.StdEncoding.DecodeString(config.Base64AuthenticationKey)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to decode %v", config.Base64AuthenticationKey))
	}

	store := sessions.NewCookieStore(authKey)
	store.Options.Path = "/"
	store.Options.HttpOnly = true
	store.Options.Secure = true

	store.Options.Domain = config.Domain
	store.Options.SameSite = http.SameSiteLaxMode

	return store, nil
}
