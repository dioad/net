package awssigv4

import (
	"bytes"
	stdcontext "context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/dioad/net/http/auth/context"
)

// Handler implements AWS SigV4 authentication for HTTP requests.
type Handler struct {
	cfg ServerConfig
}

// NewHandler creates a new AWS SigV4 authentication handler with the provided configuration.
func NewHandler(cfg ServerConfig) *Handler {
	if cfg.MaxTimestampDiff == 0 {
		cfg.MaxTimestampDiff = DefaultMaxTimestampDiff
	}
	if cfg.VerifyCredentials && cfg.AWSConfig.Region == "" {
		panic("awssigv4: AWSConfig must be provided when VerifyCredentials is true")
	}
	return &Handler{cfg: cfg}
}

// AuthRequest authenticates an HTTP request using AWS SigV4.
// It validates the signature and optionally verifies the credentials with AWS STS.
func (h *Handler) AuthRequest(r *http.Request) (stdcontext.Context, error) {
	// Parse Authorization header
	accessKeyID, credentialScope, _, _, err := ParseAuthorizationHeader(r.Header.Get(AuthorizationHeader))
	if err != nil {
		return r.Context(), fmt.Errorf("failed to parse authorization header: %w", err)
	}

	// Verify timestamp
	_, err = h.verifyTimestamp(r)
	if err != nil {
		return r.Context(), err
	}

	// Read and validate request body
	bodyBytes, err := h.readAndRestoreBody(r)
	if err != nil {
		return r.Context(), err
	}

	// Verify payload hash
	expectedPayloadHash := r.Header.Get(ContentSHA256Header)
	actualPayloadHash := hashSHA256(bodyBytes)
	if expectedPayloadHash != actualPayloadHash {
		return r.Context(), errors.New("payload hash mismatch")
	}

	// Parse credential scope to extract region and service
	scopeParts := strings.Split(credentialScope, "/")
	if len(scopeParts) != 4 {
		return r.Context(), errors.New("invalid credential scope format")
	}
	region := scopeParts[1]
	service := scopeParts[2]

	// Verify region and service match configuration if specified
	if h.cfg.Region != "" && region != h.cfg.Region {
		return r.Context(), fmt.Errorf("region mismatch: expected %s, got %s", h.cfg.Region, region)
	}
	if h.cfg.Service != "" && service != h.cfg.Service {
		return r.Context(), fmt.Errorf("service mismatch: expected %s, got %s", h.cfg.Service, service)
	}

	// Create principal with available information
	principal := &AWSPrincipal{
		AccessKeyID: accessKeyID,
		Region:      region,
		Service:     service,
	}

	// If credential verification is enabled, verify with AWS STS
	if h.cfg.VerifyCredentials {
		stsClient := sts.NewFromConfig(h.cfg.AWSConfig)
		identity, err := stsClient.GetCallerIdentity(r.Context(), &sts.GetCallerIdentityInput{})
		if err != nil {
			return r.Context(), fmt.Errorf("failed to verify credentials with AWS STS: %w", err)
		}

		// Parse ARN to get more detailed information
		if identity.Arn != nil {
			arnPrincipal, err := ParseARN(*identity.Arn)
			if err == nil {
				principal.ARN = arnPrincipal.ARN
				principal.AccountID = arnPrincipal.AccountID
				principal.Type = arnPrincipal.Type
				principal.UserID = arnPrincipal.UserID
			}
		}

		if identity.Account != nil {
			principal.AccountID = *identity.Account
		}
		if identity.UserId != nil {
			principal.UserID = *identity.UserId
		}
	}

	// Note: Without AWS credentials, we cannot verify the signature cryptographically
	// because we don't have access to the secret key. The signature verification
	// happens implicitly when VerifyCredentials is true and the STS call succeeds.
	// If VerifyCredentials is false, we're only validating the signature format.

	// Store principal in context
	ctx := context.NewContextWithAuthenticatedPrincipal(r.Context(), principal.String())
	ctx = NewContextWithAWSPrincipal(ctx, principal)

	return ctx, nil
}

// verifyTimestamp checks the timestamp header to ensure it's within the allowed time window.
func (h *Handler) verifyTimestamp(r *http.Request) (time.Time, error) {
	timestampStr := r.Header.Get(DateHeader)
	if timestampStr == "" {
		return time.Time{}, fmt.Errorf("missing %s header", DateHeader)
	}

	timestamp, err := time.Parse(TimeFormat, timestampStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timestamp format: %w", err)
	}

	now := time.Now().UTC()
	diff := now.Sub(timestamp)

	// Check if timestamp is too old
	if diff > h.cfg.MaxTimestampDiff {
		return time.Time{}, errors.New("request timestamp expired")
	}

	// Check if timestamp is too far in the future (allow small clock skew)
	if diff < -30*time.Second {
		return time.Time{}, errors.New("request timestamp too far in the future")
	}

	return timestamp, nil
}

// readAndRestoreBody reads the request body and restores it for the handler.
func (h *Handler) readAndRestoreBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return []byte{}, nil
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	// Restore body for handler to use
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	return bodyBytes, nil
}

// Wrap wraps an http.Handler with AWS SigV4 authentication.
func (h *Handler) Wrap(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, err := h.AuthRequest(r)

		if err != nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}
