package oidc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractClaimsMap(t *testing.T) {
	claims := jwt.MapClaims{
		"sub":  "1234567890",
		"name": "John Doe",
		"iat":  float64(1516239022),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("secret"))
	require.NoError(t, err)

	extracted, err := ExtractClaimsMap(tokenString)
	assert.NoError(t, err)
	assert.Equal(t, claims["sub"], extracted["sub"])
	assert.Equal(t, claims["name"], extracted["name"])
	assert.Equal(t, claims["iat"], extracted["iat"])
}

func TestExtractClaimsMap_Invalid(t *testing.T) {
	_, err := ExtractClaimsMap("invalid-token")
	assert.Error(t, err)
}

func TestDecodeTokenData(t *testing.T) {
	now := time.Now().Unix()
	claims := map[string]any{
		"sub": "1234567890",
		"exp": float64(now + 3600),
		"iat": float64(now),
		"nbf": float64(now),
	}

	payload, _ := json.Marshal(claims)
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payload)
	tokenString := fmt.Sprintf("header.%s.signature", payloadEncoded)

	data, err := decodeTokenData(tokenString)
	assert.NoError(t, err)

	dataMap, ok := data.(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "1234567890", dataMap["sub"])
	assert.NotNil(t, dataMap["exp_datetime"])
	assert.NotNil(t, dataMap["iat_datetime"])
	assert.NotNil(t, dataMap["nbf_datetime"])
}

func TestPredicateValidator(t *testing.T) {
	mockParent := &mockValidator{
		claims: map[string]any{"sub": "123"},
	}

	predicate := &ClaimKey{Key: "org", Value: "my-org"}
	validator := NewPredicateValidator(mockParent, predicate)

	// Valid token with matching claim
	claims := jwt.MapClaims{"org": "my-org"}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte("secret"))

	got, err := validator.ValidateToken(context.Background(), tokenString)
	assert.NoError(t, err)
	assert.Equal(t, mockParent.claims, got)

	// Valid token with non-matching claim
	claims2 := jwt.MapClaims{"org": "other-org"}
	token2 := jwt.NewWithClaims(jwt.SigningMethodHS256, claims2)
	tokenString2, _ := token2.SignedString([]byte("secret"))

	_, err = validator.ValidateToken(context.Background(), tokenString2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "predicate validation failed")
}

func TestMultiValidator(t *testing.T) {
	v1 := &mockValidator{err: fmt.Errorf("fail 1")}
	v2 := &mockValidator{claims: "success 2"}

	mv := NewMultiValidator(v1, v2)

	claims, err := mv.ValidateToken(context.Background(), "some-token")
	assert.NoError(t, err)
	assert.Equal(t, "success 2", claims)

	v3 := &mockValidator{err: fmt.Errorf("fail 3")}
	mv2 := NewMultiValidator(v1, v3)
	_, err = mv2.ValidateToken(context.Background(), "some-token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token validation failed")
}

func TestOIDCEndpoint_Discovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			config := OpenIDConfiguration{
				Issuer:                "http://example.com",
				AuthorizationEndpoint: "http://example.com/auth",
				TokenEndpoint:         "http://example.com/token",
			}
			json.NewEncoder(w).Encode(config)
		}
	}))
	defer server.Close()

	endpoint, err := NewEndpoint(server.URL)
	require.NoError(t, err)

	config, err := endpoint.DiscoveredConfiguration()
	assert.NoError(t, err)
	assert.Equal(t, "http://example.com", config.Issuer)
	assert.Equal(t, "http://example.com/auth", config.AuthorizationEndpoint)
}

type mockValidator struct {
	claims any
	err    error
}

func (m *mockValidator) ValidateToken(ctx context.Context, tokenString string) (any, error) {
	return m.claims, m.err
}

func (m *mockValidator) String() string {
	return "mock"
}
