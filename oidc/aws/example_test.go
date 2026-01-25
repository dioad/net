package aws_test

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/dioad/net/oidc/aws"
)

// ExampleNewTokenSource demonstrates how to use AWS AssumeRoleWithWebIdentity
// to obtain temporary AWS credentials using an OIDC token.
func ExampleNewTokenSource() {
	// Load AWS configuration (region, etc.)
	awsConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}

	// Get an OIDC token from your identity provider
	// For example, from GitHub Actions: os.Getenv("ACTIONS_ID_TOKEN")
	oidcToken := "your-oidc-token-here"

	// Create a token source that will assume an AWS role using the OIDC token
	tokenSource := aws.NewTokenSource(
		aws.WithAWSConfig(awsConfig),
		aws.WithRoleARN("arn:aws:iam::123456789012:role/MyRole"),
		aws.WithRoleSessionName("my-session"),
		aws.WithWebIdentityToken(oidcToken),
	)

	// Get temporary AWS credentials
	token, err := tokenSource.Token()
	if err != nil {
		log.Fatalf("failed to get token: %v", err)
	}

	// Decode to get AWS credentials
	creds, err := aws.DecodeCredentials(token)
	if err != nil {
		log.Fatalf("failed to decode credentials: %v", err)
	}

	fmt.Printf("Access Key ID: %s\n", creds.AccessKeyID)
	fmt.Printf("Expires: %v\n", creds.Expiration)
}

// ExampleNewHTTPClient demonstrates how to create an HTTP client that automatically
// refreshes AWS credentials using AssumeRoleWithWebIdentity.
func ExampleNewHTTPClient() {
	awsConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}

	oidcToken := "your-oidc-token-here"

	// Create an HTTP client with automatic credential refresh
	client := aws.NewHTTPClient(
		context.Background(),
		aws.WithAWSConfig(awsConfig),
		aws.WithRoleARN("arn:aws:iam::123456789012:role/MyRole"),
		aws.WithRoleSessionName("my-session"),
		aws.WithWebIdentityToken(oidcToken),
	)

	// Use the client to make requests
	// The Authorization header will contain the AWS credentials
	resp, err := client.Get("https://api.example.com/resource")
	if err != nil {
		log.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	fmt.Printf("Response status: %d\n", resp.StatusCode)
}

// ExampleNewTokenSource_withGitHubActions demonstrates using the AWS token source
// with GitHub Actions OIDC tokens.
func ExampleNewTokenSource_withGitHubActions() {
	// In a GitHub Actions workflow, you can get an OIDC token
	// by using the github-actions token source first
	ctx := context.Background()

	awsConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}

	// In real usage, you would get the GitHub Actions OIDC token like this:
	// import "github.com/dioad/net/oidc/githubactions"
	// ghTokenSource := githubactions.NewTokenSource(githubactions.WithAudience("sts.amazonaws.com"))
	// ghToken, _ := ghTokenSource.Token()
	// oidcToken := ghToken.AccessToken

	// For this example, assume we have the token
	oidcToken := "github-actions-oidc-token"

	// Use the GitHub Actions OIDC token to assume an AWS role
	awsTokenSource := aws.NewTokenSource(
		aws.WithAWSConfig(awsConfig),
		aws.WithRoleARN("arn:aws:iam::123456789012:role/GitHubActionsRole"),
		aws.WithRoleSessionName("github-actions"),
		aws.WithWebIdentityToken(oidcToken),
	)

	token, err := awsTokenSource.Token()
	if err != nil {
		log.Fatalf("failed to get AWS credentials: %v", err)
	}

	fmt.Printf("Got AWS credentials, expires at: %v\n", token.Expiry)
}
