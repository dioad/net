package hmac

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
)

func HMACKeyBytes(sharedKey, data []byte) ([]byte, error) {
	h := hmac.New(sha256.New, sharedKey)
	_, err := h.Write(data)
	if err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

func HMACKey(sharedKey, data []byte) (string, error) {
	keyBytes, err := HMACKeyBytes(sharedKey, data)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", keyBytes), nil
}
