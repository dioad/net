package oidc

import (
	"testing"
)

func FuzzDecodeTokenData(f *testing.F) {
	// A sample JWT-like string (header.payload.signature)
	f.Add("header.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.signature")
	f.Fuzz(func(t *testing.T, token string) {
		_, _ = decodeTokenData(token)
	})
}
