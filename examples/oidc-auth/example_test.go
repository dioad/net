package oidcauth_test

import (
	"fmt"
	"net/http"

	diohttp "github.com/dioad/net/http"
	"github.com/dioad/net/oidc"
)

// Example demonstrates creating an HTTP server with OIDC/JWT authentication.
func Example() {
	// Create OIDC validator configuration
	validatorConfig := oidc.ValidatorConfig{
		EndpointConfig: oidc.EndpointConfig{
			Type: "githubactions",
			URL:  "https://token.actions.githubusercontent.com",
		},
		Audiences: []string{"https://github.com/my-org"},
		Issuer:    "https://token.actions.githubusercontent.com",
	}

	// Create a simple handler
	myHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, authenticated user!")
	})

	// Create server with OIDC validator as global middleware
	config := diohttp.Config{ListenAddress: ":8080"}
	server := diohttp.NewServer(config, diohttp.WithOAuth2Validator([]oidc.ValidatorConfig{validatorConfig}))

	server.AddHandler("/secure", myHandler)

	fmt.Println("Server configured with OIDC authentication")
	// Output: Server configured with OIDC authentication
}
