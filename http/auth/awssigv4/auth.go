// Package awssigv4 provides AWS Signature Version 4 authentication middleware.
//
// This implementation follows the AWS Signature Version 4 signing process as described in:
// https://docs.aws.amazon.com/AmazonS3/latest/API/sig-v4-authenticating-requests.html
//
// It provides both client-side request signing and server-side signature verification:
//   - Client: Uses AWS SDK v2 credentials to sign requests with AWS SigV4
//   - Server: Verifies AWS SigV4 signatures and extracts AWS identity information
//
// The package exposes an AWSPrincipal containing parsed ARN information including:
// account ID, region, service, resource type, and resource name.
package awssigv4

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	// AuthScheme is the scheme used in the Authorization header
	AuthScheme = "AWS4-HMAC-SHA256"
	// AWS SigV4 headers
	AuthorizationHeader = "Authorization"
	DateHeader          = "X-Amz-Date"
	ContentSHA256Header = "X-Amz-Content-Sha256"
	SecurityTokenHeader = "X-Amz-Security-Token"

	// Time format for AWS SigV4
	TimeFormat      = "20060102T150405Z"
	ShortDateFormat = "20060102"

	// Default values
	DefaultMaxTimestampDiff = 5 * time.Minute
)

// CanonicalRequest generates the canonical request string for AWS SigV4.
// Format:
// HTTPMethod\n
// CanonicalURI\n
// CanonicalQueryString\n
// CanonicalHeaders\n
// SignedHeaders\n
// HashedPayload
func CanonicalRequest(method, uri, query, canonicalHeaders, signedHeaders, hashedPayload string) string {
	return strings.Join([]string{
		method,
		uri,
		query,
		canonicalHeaders,
		signedHeaders,
		hashedPayload,
	}, "\n")
}

// StringToSign generates the string to sign for AWS SigV4.
// Format:
// Algorithm\n
// RequestDateTime\n
// CredentialScope\n
// HashedCanonicalRequest
func StringToSign(timestamp time.Time, region, service, hashedCanonicalRequest string) string {
	credentialScope := CredentialScope(timestamp, region, service)
	return strings.Join([]string{
		AuthScheme,
		timestamp.Format(TimeFormat),
		credentialScope,
		hashedCanonicalRequest,
	}, "\n")
}

// CredentialScope generates the credential scope string.
// Format: YYYYMMDD/region/service/aws4_request
func CredentialScope(timestamp time.Time, region, service string) string {
	return fmt.Sprintf("%s/%s/%s/aws4_request",
		timestamp.Format(ShortDateFormat),
		region,
		service,
	)
}

// DeriveSigningKey generates the signing key for AWS SigV4.
func DeriveSigningKey(secretKey string, timestamp time.Time, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(timestamp.Format(ShortDateFormat)))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}

// CalculateSignature calculates the AWS SigV4 signature.
func CalculateSignature(signingKey []byte, stringToSign string) string {
	signature := hmacSHA256(signingKey, []byte(stringToSign))
	return hex.EncodeToString(signature)
}

// hmacSHA256 computes HMAC-SHA256.
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// hashSHA256 computes SHA256 hash and returns hex-encoded string.
func hashSHA256(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// CanonicalHeaders formats headers for AWS SigV4 canonical request.
// Headers are sorted, lowercased, and trimmed.
func CanonicalHeaders(headers http.Header, signedHeaders []string) string {
	var canonicalHeaders strings.Builder
	for _, h := range signedHeaders {
		canonicalKey := strings.ToLower(strings.TrimSpace(h))
		values := headers.Values(h)
		if len(values) == 0 {
			continue
		}
		// Join multiple values with comma, trim spaces
		var trimmedValues []string
		for _, v := range values {
			trimmedValues = append(trimmedValues, strings.TrimSpace(v))
		}
		canonicalValue := strings.Join(trimmedValues, ",")
		canonicalHeaders.WriteString(canonicalKey)
		canonicalHeaders.WriteString(":")
		canonicalHeaders.WriteString(canonicalValue)
		canonicalHeaders.WriteString("\n")
	}
	return canonicalHeaders.String()
}

// SignedHeadersList returns the semicolon-separated list of signed header names.
func SignedHeadersList(headers []string) string {
	normalized := make([]string, len(headers))
	for i, h := range headers {
		normalized[i] = strings.ToLower(strings.TrimSpace(h))
	}
	sort.Strings(normalized)
	return strings.Join(normalized, ";")
}

// CanonicalQueryString formats query parameters for AWS SigV4 canonical request.
func CanonicalQueryString(query string) string {
	if query == "" {
		return ""
	}
	// Parse and sort query parameters
	params := strings.Split(query, "&")
	sort.Strings(params)
	return strings.Join(params, "&")
}

// ParseAuthorizationHeader parses the AWS SigV4 Authorization header.
// Expected format: AWS4-HMAC-SHA256 Credential=AccessKeyId/scope, SignedHeaders=..., Signature=...
func ParseAuthorizationHeader(authHeader string) (accessKeyID, credentialScope, signedHeaders, signature string, err error) {
	if !strings.HasPrefix(authHeader, AuthScheme+" ") {
		return "", "", "", "", fmt.Errorf("invalid authorization scheme")
	}

	// Remove the scheme prefix
	authParams := strings.TrimPrefix(authHeader, AuthScheme+" ")

	// Split by comma to get individual components
	parts := strings.Split(authParams, ",")
	if len(parts) != 3 {
		return "", "", "", "", fmt.Errorf("invalid authorization header format")
	}

	// Parse Credential
	credentialPart := strings.TrimSpace(parts[0])
	if !strings.HasPrefix(credentialPart, "Credential=") {
		return "", "", "", "", fmt.Errorf("missing Credential in authorization header")
	}
	credential := strings.TrimPrefix(credentialPart, "Credential=")
	credentialParts := strings.SplitN(credential, "/", 2)
	if len(credentialParts) != 2 {
		return "", "", "", "", fmt.Errorf("invalid credential format")
	}
	accessKeyID = credentialParts[0]
	credentialScope = credentialParts[1]

	// Parse SignedHeaders
	signedHeadersPart := strings.TrimSpace(parts[1])
	if !strings.HasPrefix(signedHeadersPart, "SignedHeaders=") {
		return "", "", "", "", fmt.Errorf("missing SignedHeaders in authorization header")
	}
	signedHeaders = strings.TrimPrefix(signedHeadersPart, "SignedHeaders=")

	// Parse Signature
	signaturePart := strings.TrimSpace(parts[2])
	if !strings.HasPrefix(signaturePart, "Signature=") {
		return "", "", "", "", fmt.Errorf("missing Signature in authorization header")
	}
	signature = strings.TrimPrefix(signaturePart, "Signature=")

	return accessKeyID, credentialScope, signedHeaders, signature, nil
}
