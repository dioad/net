package http

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/sessions"
	"github.com/pkg/errors"
)

type CookieConfig struct {
	Base64AuthenticationKey string `mapstructure:"base64-authentication-key"`
	Base64EncryptionKey     string `mapstructure:"base64-encryption-key"`
	MaxAge                  int    `mapstructure:"max-age"`
	Domain                  string `mapstructure:"domain"`
}

func NewPersistentCookieStore(config CookieConfig) (*sessions.CookieStore, error) {
	store, err := NewSessionCookieStore(config)
	if err != nil {
		return nil, err
	}
	store.MaxAge(config.MaxAge)

	return store, nil
}

func NewSessionCookieStore(config CookieConfig) (*sessions.CookieStore, error) {
	authKey, err := decodeBase64String(config.Base64AuthenticationKey)
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

func decodeBase64String(raw string) ([]byte, error) {
	b := make([]byte, 0)
	r := strings.NewReader(raw)
	d := base64.NewDecoder(base64.StdEncoding, r)
	_, err := d.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}
