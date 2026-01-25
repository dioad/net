package awssigv4_test

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/dioad/net/http/auth/awssigv4"
)

// ExampleClientAuth demonstrates how to use AWS SigV4 authentication on the client side.
func ExampleClientAuth() {
	// Load AWS configuration from environment or instance profile
	awsConfig, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
	)
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}

	// Create client auth
	clientAuth := awssigv4.NewClientAuth(awsConfig, "us-east-1", "execute-api")

	// Create and sign a request
	req, err := http.NewRequest("GET", "https://api.example.com/resource", nil)
	if err != nil {
		log.Fatalf("failed to create request: %v", err)
	}

	if err := clientAuth.AddAuth(req); err != nil {
		log.Fatalf("failed to sign request: %v", err)
	}

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	fmt.Printf("Response status: %d\n", resp.StatusCode)
}

// ExampleClientAuth_HTTPClient demonstrates using the HTTPClient method to get a pre-configured client.
func ExampleClientAuth_HTTPClient() {
	awsConfig, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
	)
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}

	clientAuth := awssigv4.NewClientAuth(awsConfig, "us-east-1", "execute-api")

	// Get an HTTP client that automatically signs requests
	client := clientAuth.HTTPClient()

	// All requests made with this client will be automatically signed
	resp, err := client.Get("https://api.example.com/resource")
	if err != nil {
		log.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	fmt.Printf("Response status: %d\n", resp.StatusCode)
}

// ExampleNewHandler demonstrates how to use AWS SigV4 authentication on the server side.
func ExampleNewHandler() {
	// Create server handler
	handler := awssigv4.NewHandler(awssigv4.ServerConfig{
		CommonConfig: awssigv4.CommonConfig{
			Region:  "us-east-1",
			Service: "execute-api",
		},
		VerifyCredentials: false, // Set to true to verify with AWS STS
	})

	// Wrap your HTTP handler
	protectedHandler := handler.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract AWS principal from context
		awsPrincipal := awssigv4.AWSPrincipalFromContext(r.Context())
		if awsPrincipal != nil {
			fmt.Fprintf(w, "Authenticated as: %s\n", awsPrincipal.AccessKeyID)
			fmt.Fprintf(w, "Account ID: %s\n", awsPrincipal.AccountID)
			fmt.Fprintf(w, "Region: %s\n", awsPrincipal.Region)
		}
	}))

	// Use the protected handler
	http.Handle("/api/", protectedHandler)
}

// ExampleAWSPrincipal demonstrates working with AWS principal information.
func ExampleAWSPrincipal() {
	// This would typically come from the request context after authentication
	principal := &awssigv4.AWSPrincipal{
		AccessKeyID: "AKIAIOSFODNN7EXAMPLE",
		ARN:         "arn:aws:sts::123456789012:assumed-role/MyRole/session-name",
		AccountID:   "123456789012",
		Type:        "assumed-role",
		UserID:      "MyRole/session-name",
		Region:      "us-east-1",
		Service:     "sts",
	}

	fmt.Printf("Principal: %s\n", principal.String())
	fmt.Printf("Is assumed role: %v\n", principal.IsAssumedRole())
	fmt.Printf("Is user: %v\n", principal.IsUser())
	fmt.Printf("Account ID: %s\n", principal.AccountID)

	// Output:
	// Principal: arn:aws:sts::123456789012:assumed-role/MyRole/session-name
	// Is assumed role: true
	// Is user: false
	// Account ID: 123456789012
}

// ExampleParseARN demonstrates parsing AWS ARNs.
func ExampleParseARN() {
	arn := "arn:aws:iam::123456789012:user/alice"
	principal, err := awssigv4.ParseARN(arn)
	if err != nil {
		log.Fatalf("failed to parse ARN: %v", err)
	}

	fmt.Printf("Account ID: %s\n", principal.AccountID)
	fmt.Printf("Type: %s\n", principal.Type)
	fmt.Printf("User ID: %s\n", principal.UserID)
	fmt.Printf("Service: %s\n", principal.Service)

	// Output:
	// Account ID: 123456789012
	// Type: user
	// User ID: alice
	// Service: iam
}
