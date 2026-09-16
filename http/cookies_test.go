package http

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testAuthKey() string {
	return base64.StdEncoding.EncodeToString(make([]byte, 32))
}

func TestNewPersistentCookieStore_DefaultsMaxAgeWhenUnset(t *testing.T) {
	store, err := NewPersistentCookieStore(CookieConfig{
		Base64AuthenticationKey: testAuthKey(),
	})
	require.NoError(t, err)

	assert.Equal(t, DefaultPersistentCookieMaxAge, store.Options.MaxAge, "an unset MaxAge must default to a persistent lifetime, not session-only (MaxAge=0)")
}

func TestNewPersistentCookieStore_RespectsExplicitMaxAge(t *testing.T) {
	store, err := NewPersistentCookieStore(CookieConfig{
		Base64AuthenticationKey: testAuthKey(),
		MaxAge:                  3600,
	})
	require.NoError(t, err)

	assert.Equal(t, 3600, store.Options.MaxAge)
}

func TestNewPersistentCookieStore_RespectsExplicitNegativeMaxAge(t *testing.T) {
	store, err := NewPersistentCookieStore(CookieConfig{
		Base64AuthenticationKey: testAuthKey(),
		MaxAge:                  -1,
	})
	require.NoError(t, err)

	assert.Equal(t, -1, store.Options.MaxAge, "an explicit negative MaxAge (gorilla's documented 'delete cookie immediately') must not be overridden by the default")
}
