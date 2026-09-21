package http

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/sessions"
)

// CookieKeyPair is one (authentication, encryption) key pair used to sign
// and, optionally, encrypt session cookies.
type CookieKeyPair struct {
	Base64AuthenticationKey string `mapstructure:"base64-authentication-key"`
	// Base64EncryptionKey is optional. Unset, cookie contents are signed
	// but not encrypted, which keeps them inspectable for local debugging.
	// When set, it must decode to 16, 24, or 32 bytes to select AES-128,
	// AES-192, or AES-256.
	Base64EncryptionKey string `mapstructure:"base64-encryption-key"`
}

// CookieConfig describes the configuration for HTTP cookies.
type CookieConfig struct {
	// KeyPairs is ordered newest-first: the first pair signs/encrypts new
	// cookies, and every pair is tried when decoding, so old pairs can be
	// kept around during a rotation window and then dropped once no
	// outstanding cookie could still be using them. At least one pair is
	// required.
	KeyPairs []CookieKeyPair `mapstructure:"key-pairs"`
	MaxAge   int             `mapstructure:"max-age"`
	Domain   string          `mapstructure:"domain"`
}

// DefaultPersistentCookieMaxAge is used by NewPersistentCookieStore when
// CookieConfig.MaxAge is left at its zero value.
const DefaultPersistentCookieMaxAge = 30 * 24 * 60 * 60 // 30 days, in seconds

// NewPersistentCookieStore creates a persistent cookie store from the provided configuration.
func NewPersistentCookieStore(config CookieConfig) (*sessions.CookieStore, error) {
	store, err := NewSessionCookieStore(config)
	if err != nil {
		return nil, err
	}

	maxAge := config.MaxAge
	if maxAge == 0 {
		maxAge = DefaultPersistentCookieMaxAge
	}
	store.MaxAge(maxAge)

	return store, nil
}

// NewSessionCookieStore creates a session cookie store from the provided configuration.
func NewSessionCookieStore(config CookieConfig) (*sessions.CookieStore, error) {
	if len(config.KeyPairs) == 0 {
		return nil, errors.New("no cookie key pairs configured")
	}

	// sessions.NewCookieStore takes a flat (auth, encryption) sequence; the
	// encryption slot can be nil for any pair, not only the last, per
	// securecookie.CodecsFromPairs.
	keys := make([][]byte, 0, len(config.KeyPairs)*2)
	for i, kp := range config.KeyPairs {
		authKey, err := base64.StdEncoding.DecodeString(kp.Base64AuthenticationKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decode authentication key for key pair %d: %w", i, err)
		}

		var encKey []byte
		if kp.Base64EncryptionKey != "" {
			encKey, err = base64.StdEncoding.DecodeString(kp.Base64EncryptionKey)
			if err != nil {
				return nil, fmt.Errorf("failed to decode encryption key for key pair %d: %w", i, err)
			}
		}

		keys = append(keys, authKey, encKey)
	}

	store := sessions.NewCookieStore(keys...)
	store.Options.Path = "/"
	store.Options.HttpOnly = true
	store.Options.Secure = true

	store.Options.Domain = config.Domain
	store.Options.SameSite = http.SameSiteLaxMode

	return store, nil
}
