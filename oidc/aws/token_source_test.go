package aws

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"golang.org/x/oauth2"
)

func TestEncodeDecodeCredentials(t *testing.T) {
	// Create test credentials
	expiration := time.Now().Add(1 * time.Hour)
	testCreds := &types.Credentials{
		AccessKeyId:     aws.String("ASIA-TEST-ACCESS-KEY"),
		SecretAccessKey: aws.String("test-secret-key"),
		SessionToken:    aws.String("test-session-token"),
		Expiration:      &expiration,
	}

	// Encode credentials
	token, err := encodeCredentials(testCreds)
	if err != nil {
		t.Fatalf("failed to encode credentials: %v", err)
	}

	if token.TokenType != "aws-credentials" {
		t.Errorf("expected token type 'aws-credentials', got %s", token.TokenType)
	}

	if !token.Expiry.Equal(expiration) {
		t.Errorf("expected expiry %v, got %v", expiration, token.Expiry)
	}

	// Decode credentials
	decoded, err := DecodeCredentials(token)
	if err != nil {
		t.Fatalf("failed to decode credentials: %v", err)
	}

	if decoded.AccessKeyID != *testCreds.AccessKeyId {
		t.Errorf("expected access key %s, got %s", *testCreds.AccessKeyId, decoded.AccessKeyID)
	}

	if decoded.SecretAccessKey != *testCreds.SecretAccessKey {
		t.Errorf("expected secret key %s, got %s", *testCreds.SecretAccessKey, decoded.SecretAccessKey)
	}

	if decoded.SessionToken != *testCreds.SessionToken {
		t.Errorf("expected session token %s, got %s", *testCreds.SessionToken, decoded.SessionToken)
	}

	if !decoded.Expiration.Equal(expiration) {
		t.Errorf("expected expiration %v, got %v", expiration, decoded.Expiration)
	}
}

func TestDecodeCredentials_InvalidTokenType(t *testing.T) {
	token := &oauth2.Token{
		AccessToken: "invalid",
		TokenType:   "bearer",
	}

	_, err := DecodeCredentials(token)
	if err == nil {
		t.Error("expected error for invalid token type, got nil")
	}
}

func TestTokenSource_MissingRoleARN(t *testing.T) {
	ts := &tokenSource{
		webIdentityToken: "test-token",
	}

	_, err := ts.Token()
	if err == nil {
		t.Error("expected error for missing role ARN, got nil")
	}
}

func TestTokenSource_MissingWebIdentityToken(t *testing.T) {
	ts := &tokenSource{
		roleARN: "arn:aws:iam::123456789012:role/TestRole",
	}

	_, err := ts.Token()
	if err == nil {
		t.Error("expected error for missing web identity token, got nil")
	}
}

func TestClaimsValidate(t *testing.T) {
	claims := &Claims{}
	err := claims.Validate(context.Background())
	if err != nil {
		t.Errorf("Validate() should not return error, got: %v", err)
	}
}

func TestWithRoleARN(t *testing.T) {
	ts := &tokenSource{}
	opt := WithRoleARN("arn:aws:iam::123456789012:role/TestRole")
	opt(ts)

	if ts.roleARN != "arn:aws:iam::123456789012:role/TestRole" {
		t.Errorf("expected role ARN to be set, got %s", ts.roleARN)
	}
}

func TestWithRoleSessionName(t *testing.T) {
	ts := &tokenSource{}
	opt := WithRoleSessionName("test-session")
	opt(ts)

	if ts.roleSessionName != "test-session" {
		t.Errorf("expected session name to be set, got %s", ts.roleSessionName)
	}
}

func TestWithWebIdentityToken(t *testing.T) {
	ts := &tokenSource{}
	opt := WithWebIdentityToken("test-token")
	opt(ts)

	if ts.webIdentityToken != "test-token" {
		t.Errorf("expected web identity token to be set, got %s", ts.webIdentityToken)
	}
}
