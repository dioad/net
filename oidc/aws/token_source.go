package aws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"golang.org/x/oauth2"
)

type tokenSource struct {
	roleARN          string
	roleSessionName  string
	webIdentityToken string
	awsConfig        aws.Config
	currentToken     *oauth2.Token
}

// Opt is a function option for configuring the token source.
type Opt func(*tokenSource)

// WithRoleARN sets the ARN of the role to assume.
func WithRoleARN(roleARN string) Opt {
	return func(ts *tokenSource) {
		ts.roleARN = roleARN
	}
}

// WithRoleSessionName sets the session name for the assumed role.
func WithRoleSessionName(sessionName string) Opt {
	return func(ts *tokenSource) {
		ts.roleSessionName = sessionName
	}
}

// WithWebIdentityToken sets the web identity token (OIDC token) to use for assuming the role.
func WithWebIdentityToken(token string) Opt {
	return func(ts *tokenSource) {
		ts.webIdentityToken = token
	}
}

// WithAWSConfig sets the AWS configuration to use.
func WithAWSConfig(cfg aws.Config) Opt {
	return func(ts *tokenSource) {
		ts.awsConfig = cfg
	}
}

// NewTokenSource creates a new token source for AWS AssumeRoleWithWebIdentity.
// It uses a web identity token (OIDC token) to assume an AWS IAM role and obtain
// temporary AWS credentials, which are then used to create an OAuth2 token.
//
// This is useful for authenticating to AWS services from environments where you
// have an OIDC token (e.g., GitHub Actions, Kubernetes service accounts, etc.)
// and want to assume an AWS role without long-lived credentials.
//
// See: https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRoleWithWebIdentity.html
func NewTokenSource(opts ...Opt) oauth2.TokenSource {
	source := &tokenSource{
		roleSessionName: "oidc-session",
	}
	for _, opt := range opts {
		opt(source)
	}

	return oauth2.ReuseTokenSource(nil, source)
}

// Token retrieves AWS temporary credentials by calling AssumeRoleWithWebIdentity.
func (ts *tokenSource) Token() (*oauth2.Token, error) {
	if ts.roleARN == "" {
		return nil, fmt.Errorf("role ARN is required")
	}
	if ts.webIdentityToken == "" {
		return nil, fmt.Errorf("web identity token is required")
	}

	// Create STS client
	stsClient := sts.NewFromConfig(ts.awsConfig)

	// Call AssumeRoleWithWebIdentity
	input := &sts.AssumeRoleWithWebIdentityInput{
		RoleArn:          aws.String(ts.roleARN),
		RoleSessionName:  aws.String(ts.roleSessionName),
		WebIdentityToken: aws.String(ts.webIdentityToken),
	}

	result, err := stsClient.AssumeRoleWithWebIdentity(context.Background(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to assume role with web identity: %w", err)
	}

	if result.Credentials == nil {
		return nil, fmt.Errorf("no credentials returned from AssumeRoleWithWebIdentity")
	}

	// Create an OAuth2 token with the AWS credentials encoded as a JWT-like format
	// This allows the credentials to be used with the oauth2.Client pattern
	token, err := encodeCredentials(result.Credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to encode credentials: %w", err)
	}

	return token, nil
}

// encodeCredentials creates an OAuth2 token from AWS credentials.
// The access token is a base64-encoded JSON representation of the credentials.
func encodeCredentials(creds *types.Credentials) (*oauth2.Token, error) {
	if creds.AccessKeyId == nil || creds.SecretAccessKey == nil || creds.SessionToken == nil {
		return nil, fmt.Errorf("incomplete credentials")
	}

	// Create a credentials payload
	credPayload := map[string]string{
		"access_key_id":     *creds.AccessKeyId,
		"secret_access_key": *creds.SecretAccessKey,
		"session_token":     *creds.SessionToken,
	}

	// Encode as JSON
	jsonData, err := json.Marshal(credPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Base64 encode for safe transport
	encodedToken := base64.StdEncoding.EncodeToString(jsonData)

	var expiry time.Time
	if creds.Expiration != nil {
		expiry = *creds.Expiration
	}

	return &oauth2.Token{
		AccessToken: encodedToken,
		Expiry:      expiry,
		TokenType:   "aws-credentials",
	}, nil
}

// DecodeCredentials extracts AWS credentials from an OAuth2 token.
// This is a helper function for consumers who need the raw AWS credentials.
func DecodeCredentials(token *oauth2.Token) (*AWSCredentials, error) {
	if token.TokenType != "aws-credentials" {
		return nil, fmt.Errorf("invalid token type: %s", token.TokenType)
	}

	// Decode the base64-encoded token
	jsonData, err := base64.StdEncoding.DecodeString(token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decode token: %w", err)
	}

	// Unmarshal the JSON
	var credPayload map[string]string
	if err := json.Unmarshal(jsonData, &credPayload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}

	return &AWSCredentials{
		AccessKeyID:     credPayload["access_key_id"],
		SecretAccessKey: credPayload["secret_access_key"],
		SessionToken:    credPayload["session_token"],
		Expiration:      token.Expiry,
	}, nil
}

// AWSCredentials represents AWS temporary credentials.
type AWSCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Expiration      time.Time
}
