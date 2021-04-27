package hmac

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io"
)

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
