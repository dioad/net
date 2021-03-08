package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io"
)

type HMACAuthCommonConfig struct {
	// Inline shared key used to HMAC with value from HTTPHeader
	SharedKey string `mapstructure:"shared-key"`
	// Path to read shared key from
	SharedKeyPath string `mapstructure:"shared-key-path"`
}

type HMACAuthClientConfig struct {
	HMACAuthCommonConfig `mapstructure:",squash"`
}

type HMACAuthServerConfig struct {
	HMACAuthCommonConfig `mapstructure:",squash"`
	// HTTP Header to use as data input
	HTTPHeader string `mapstructure:"http-header"`
}

func HMACKeyBytes(sharedKey, data string) ([]byte, error) {
	h := hmac.New(sha256.New, []byte(sharedKey))
	_, err := io.WriteString(h, data)
	if err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

func HMACKey(sharedKey, data string) (string, error) {
	keyBytes, err := HMACKeyBytes(sharedKey, data)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", keyBytes), nil
}
