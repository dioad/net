package http

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testKeyBytes(fill byte, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = fill
	}
	return base64.StdEncoding.EncodeToString(b)
}

func testAuthKey() string  { return testKeyBytes(0xA1, 32) }
func testAuthKey2() string { return testKeyBytes(0xA2, 32) }
func testEncKey() string   { return testKeyBytes(0xE1, 32) } // AES-256

func TestNewPersistentCookieStore_DefaultsMaxAgeWhenUnset(t *testing.T) {
	store, err := NewPersistentCookieStore(CookieConfig{
		KeyPairs: []CookieKeyPair{{Base64AuthenticationKey: testAuthKey()}},
	})
	require.NoError(t, err)

	assert.Equal(t, DefaultPersistentCookieMaxAge, store.Options.MaxAge, "an unset MaxAge must default to a persistent lifetime, not session-only (MaxAge=0)")
}

func TestNewPersistentCookieStore_RespectsExplicitMaxAge(t *testing.T) {
	store, err := NewPersistentCookieStore(CookieConfig{
		KeyPairs: []CookieKeyPair{{Base64AuthenticationKey: testAuthKey()}},
		MaxAge:   3600,
	})
	require.NoError(t, err)

	assert.Equal(t, 3600, store.Options.MaxAge)
}

func TestNewPersistentCookieStore_RespectsExplicitNegativeMaxAge(t *testing.T) {
	store, err := NewPersistentCookieStore(CookieConfig{
		KeyPairs: []CookieKeyPair{{Base64AuthenticationKey: testAuthKey()}},
		MaxAge:   -1,
	})
	require.NoError(t, err)

	assert.Equal(t, -1, store.Options.MaxAge, "an explicit negative MaxAge (gorilla's documented 'delete cookie immediately') must not be overridden by the default")
}

func TestNewSessionCookieStore_NoKeyPairsErrors(t *testing.T) {
	_, err := NewSessionCookieStore(CookieConfig{})
	assert.Error(t, err)
}

// saveAndCaptureCookie saves a session with the given values through store
// and returns the raw Set-Cookie header value written to the response.
func saveAndCaptureCookie(t *testing.T, store *sessions.CookieStore, values map[string]any) string {
	t.Helper()

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	session, err := store.New(req, "session")
	require.NoError(t, err)
	for k, v := range values {
		session.Values[k] = v
	}
	require.NoError(t, store.Save(req, w, session))

	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)
	return cookies[0].Value
}

func TestNewSessionCookieStore_EncryptionIsOptional(t *testing.T) {
	decodeAuthOnly := func(t *testing.T, name, rawCookie string) error {
		t.Helper()
		// A store built with only the auth key (no encryption key)
		// authenticates a cookie's HMAC but never attempts decryption --
		// exactly the "signed but not encrypted" boundary this test checks.
		authOnlyStore, err := NewSessionCookieStore(CookieConfig{
			KeyPairs: []CookieKeyPair{{Base64AuthenticationKey: testAuthKey()}},
		})
		require.NoError(t, err)

		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: name, Value: rawCookie})
		_, err = authOnlyStore.Get(req, name)
		return err
	}

	t.Run("without an encryption key, an auth-only codec can still decode it", func(t *testing.T) {
		store, err := NewSessionCookieStore(CookieConfig{
			KeyPairs: []CookieKeyPair{{Base64AuthenticationKey: testAuthKey()}},
		})
		require.NoError(t, err)

		raw := saveAndCaptureCookie(t, store, map[string]any{"marker": "plaintext-marker-value"})
		assert.NoError(t, decodeAuthOnly(t, "session", raw), "with no encryption key, the cookie must be signed-only and decodable/inspectable with just the auth key -- required to keep contents readable for local debugging")
	})

	t.Run("with an encryption key, an auth-only codec cannot decode it", func(t *testing.T) {
		store, err := NewSessionCookieStore(CookieConfig{
			KeyPairs: []CookieKeyPair{{
				Base64AuthenticationKey: testAuthKey(),
				Base64EncryptionKey:     testEncKey(),
			}},
		})
		require.NoError(t, err)

		raw := saveAndCaptureCookie(t, store, map[string]any{"marker": "plaintext-marker-value"})
		assert.Error(t, decodeAuthOnly(t, "session", raw), "an encryption key must actually encrypt cookie contents, not just sign them")
	})
}

func TestNewSessionCookieStore_SupportsKeyRotation(t *testing.T) {
	oldAuth := testAuthKey()
	newAuth := testAuthKey2()

	oldStore, err := NewSessionCookieStore(CookieConfig{
		KeyPairs: []CookieKeyPair{{Base64AuthenticationKey: oldAuth}},
	})
	require.NoError(t, err)
	rawCookie := saveAndCaptureCookie(t, oldStore, map[string]any{"user": "alice"})

	// Rotated config: the new pair signs new cookies, but the old pair is
	// still present so a cookie issued before rotation still decodes.
	rotatedStore, err := NewSessionCookieStore(CookieConfig{
		KeyPairs: []CookieKeyPair{
			{Base64AuthenticationKey: newAuth},
			{Base64AuthenticationKey: oldAuth},
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: rawCookie})

	session, err := rotatedStore.Get(req, "session")
	require.NoError(t, err, "a cookie signed with the old (now-secondary) key pair must still decode after rotation")
	assert.Equal(t, "alice", session.Values["user"])
}
