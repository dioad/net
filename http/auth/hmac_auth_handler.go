package auth

import (
	"net/http"
	"strings"
)

func HMACAuthHandlerFunc(next http.HandlerFunc, sharedKey string, principalHeader string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			http.Error(w, "hmac auth requires SSL", http.StatusForbidden)
			return
		}

		authHeader := r.Header.Get("Authorization")
		authPrincipal := r.Header.Get(principalHeader)

		if authHeader == "" {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		if authPrincipal == "" {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		authParts := strings.Split(authHeader, " ")
		authType := authParts[0]
		authToken := authParts[1]

		if !(authType == "bearer" || authType == "token") {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		verificationKey, err := HMACKey(sharedKey, authPrincipal)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		// Check build keys.
		// TODO: change to use hmac.Equal if ! hmac.Equal(agentInfo.AuthKey, key) {
		if authToken != verificationKey {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}
}
