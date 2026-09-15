package tls

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validSelfSignedConfig() SelfSignedConfig {
	return SelfSignedConfig{
		Subject:  CertificateSubject{CommonName: "test"},
		SAN:      SANConfig{DNSNames: []string{"localhost"}},
		Duration: "5m",
		Bits:     1024,
	}
}

func TestCreateSelfSignedKeyPair(t *testing.T) {
	t.Run("returns a certificate and a non-nil pool containing it", func(t *testing.T) {
		cert, pool, err := CreateSelfSignedKeyPair(validSelfSignedConfig())
		require.NoError(t, err)
		require.NotNil(t, cert)
		require.NotNil(t, pool)
	})

	t.Run("propagates a key generation error", func(t *testing.T) {
		c := validSelfSignedConfig()
		c.Bits = 0

		_, _, err := CreateSelfSignedKeyPair(c)
		assert.Error(t, err)
	})

	t.Run("propagates a template conversion error", func(t *testing.T) {
		c := validSelfSignedConfig()
		c.Duration = "not-a-duration"

		_, _, err := CreateSelfSignedKeyPair(c)
		assert.Error(t, err)
	})
}
