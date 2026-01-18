// Package hmac provides HMAC-based authentication middleware.
package hmac

import (
	"bytes"
	stdcontext "context"
	"crypto/hmac"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dioad/net/http/auth/context"
)

// NewHandler creates a new HMAC authentication handler with the provided configuration.
func NewHandler(cfg ServerConfig) *Handler {
	if cfg.MaxTimestampDiff == 0 {
		cfg.MaxTimestampDiff = 5 * time.Minute
	}
	return &Handler{cfg: cfg}
}

// Handler implements HMAC-based authentication.
type Handler struct {
	cfg ServerConfig
}

// AuthRequest authenticates an HTTP request using HMAC.
// It expects an Authorization header in the format "HMAC principal:signature".
// It also verifies the request timestamp to prevent replay attacks.
func (a *Handler) AuthRequest(r *http.Request) (stdcontext.Context, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return r.Context(), errors.New("missing auth header")
	}

	if !strings.HasPrefix(authHeader, AuthScheme+" ") {
		return r.Context(), errors.New("invalid auth scheme")
	}

	credentials := strings.TrimPrefix(authHeader, AuthScheme+" ")
	parts := strings.SplitN(credentials, ":", 2)
	if len(parts) != 2 {
		return r.Context(), errors.New("invalid authorization header format")
	}

	principal := parts[0]
	signature := parts[1]

	// Verify timestamp
	timestampHeader := a.cfg.TimestampHeader
	if timestampHeader == "" {
		timestampHeader = DefaultTimestampHeader
	}

	timestampStr := r.Header.Get(timestampHeader)
	if timestampStr == "" {
		return r.Context(), fmt.Errorf("missing timestamp header: %s", timestampHeader)
	}

	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return r.Context(), errors.New("invalid timestamp format")
	}

	now := time.Now().Unix()
	if math.Abs(float64(now-timestamp)) > a.cfg.MaxTimestampDiff.Seconds() {
		return r.Context(), errors.New("request timestamp expired or too far in the future")
	}

	// Get signed headers
	signedHeadersStr := r.Header.Get(DefaultSignedHeadersHeader)
	var signedHeaders []string
	if signedHeadersStr != "" {
		signedHeaders = strings.Split(signedHeadersStr, ",")
	}
	// Validate signed headers against server configuration, if configured.
	if len(a.cfg.SignedHeaders) > 0 {
		if len(signedHeaders) == 0 {
			return r.Context(), errors.New("missing signed headers")
		}
		if len(signedHeaders) != len(a.cfg.SignedHeaders) {
			return r.Context(), errors.New("signed headers do not match server configuration")
		}
		for i, requiredHeader := range a.cfg.SignedHeaders {
			clientHeader := signedHeaders[i]
			if strings.ToLower(strings.TrimSpace(requiredHeader)) != strings.ToLower(strings.TrimSpace(clientHeader)) {
				return r.Context(), errors.New("signed headers do not match server configuration")
			}
		}
	}

	// Read the request body for HMAC verification
	bodyBytes := []byte{}
	if r.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			return r.Context(), fmt.Errorf("failed to read request body: %w", err)
		}
		// Restore body for handler to use
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	// Reconstruct canonical data
	verificationData := CanonicalData(r, principal, timestampStr, signedHeaders, bodyBytes)

	// Verify HMAC token
	verificationKey, err := HMACKey([]byte(a.cfg.SharedKey), []byte(verificationData))
	if err != nil {
		return r.Context(), fmt.Errorf("failed to generate verification key: %w", err)
	}

	// Use constant-time comparison to prevent timing attacks
	if !hmac.Equal([]byte(signature), []byte(verificationKey)) {
		return r.Context(), errors.New("invalid auth token")
	}

	ctx := context.NewContextWithAuthenticatedPrincipal(r.Context(), principal)

	return ctx, nil
}

func (a *Handler) Wrap(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, err := a.AuthRequest(r)

		if err != nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}
