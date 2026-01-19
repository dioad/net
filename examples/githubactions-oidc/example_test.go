package githubactions_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/dioad/net/oidc"
	"github.com/dioad/net/oidc/githubactions"
)

// Example_tokenSource demonstrates retrieving a GitHub Actions OIDC token.
func Example_tokenSource() {
	// Get the audience from environment or use default
	audience := os.Getenv("OIDC_AUDIENCE")
	if audience == "" {
		audience = "https://github.com/dioad"
	}

	// Create a token source
	tokenSource := githubactions.NewTokenSource(githubactions.WithAudience(audience))

	// Get a token
	token, err := tokenSource.Token()
	if err != nil {
		fmt.Printf("Failed to get token: %v\n", err)
		return
	}

	fmt.Printf("Token retrieved successfully\n")
	fmt.Printf("Token type: %s\n", token.TokenType)

	// Decode and display claims
	tokenParts := strings.Split(token.AccessToken, ".")
	if len(tokenParts) != 3 {
		fmt.Printf("Invalid token format\n")
		return
	}

	payload, err := base64.RawURLEncoding.DecodeString(tokenParts[1])
	if err != nil {
		fmt.Printf("Failed to decode token payload: %v\n", err)
		return
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		fmt.Printf("Failed to unmarshal claims: %v\n", err)
		return
	}

	// Verify expected claims
	expectedClaims := []string{"iss", "sub", "aud", "exp", "iat"}
	for _, claim := range expectedClaims {
		if _, ok := claims[claim]; !ok {
			fmt.Printf("Missing expected claim: %s\n", claim)
			return
		}
	}

	fmt.Printf("All expected claims present\n")
}

// Example_validator demonstrates validating a GitHub Actions OIDC token.
func Example_validator() {
	ctx := context.Background()

	// Get the audience from environment or use default
	audience := os.Getenv("OIDC_AUDIENCE")
	if audience == "" {
		audience = "https://github.com/dioad"
	}

	// Create a token source to get a token
	tokenSource := githubactions.NewTokenSource(githubactions.WithAudience(audience))
	token, err := tokenSource.Token()
	if err != nil {
		fmt.Printf("Failed to get token: %v\n", err)
		return
	}

	// Create a validator config
	validatorConfig := oidc.ValidatorConfig{
		EndpointConfig: oidc.EndpointConfig{
			Type: "githubactions",
			URL:  "https://token.actions.githubusercontent.com",
		},
		Audiences: []string{audience},
		Issuer:    "https://token.actions.githubusercontent.com",
	}

	// Create a validator
	validator, err := oidc.NewValidatorFromConfig(&validatorConfig)
	if err != nil {
		fmt.Printf("Failed to create validator: %v\n", err)
		return
	}

	// Validate the token
	claims, err := validator.ValidateToken(ctx, token.AccessToken)
	if err != nil {
		fmt.Printf("Failed to validate token: %v\n", err)
		return
	}

	fmt.Printf("Token validated successfully\n")
	fmt.Printf("Claims type: %T\n", claims)
}
