package githubactions

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDecodeToken(t *testing.T) {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":        "repo:org/repo:ref:refs/heads/main",
		"exp":        float64(now + 3600),
		"repository": "org/repo",
	}

	payload, _ := json.Marshal(claims)
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payload)
	tokenString := fmt.Sprintf("header.%s.signature", payloadEncoded)

	token, err := decodeToken(tokenString)
	assert.NoError(t, err)
	assert.Equal(t, tokenString, token.AccessToken)
	assert.Equal(t, time.Unix(now+3600, 0).Unix(), token.Expiry.Unix())
}

func TestDecodeToken_Invalid(t *testing.T) {
	_, err := decodeToken("not.a.jwt")
	assert.Error(t, err)
}
