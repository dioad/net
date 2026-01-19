package basicauth_test

import (
	"fmt"
	"net/http"

	diohttp "github.com/dioad/net/http"
	"github.com/dioad/net/http/auth/basic"
)

// Example demonstrates creating an HTTP server with basic authentication.
func Example() {
	// Create a basic auth map
	authMap := basic.AuthMap{}
	authMap.AddUserWithPlainPassword("user1", "password1")

	// Create auth handler
	authHandler, err := basic.NewHandlerWithMap(authMap)
	if err != nil {
		fmt.Printf("Error creating auth handler: %v\n", err)
		return
	}

	// Create a simple handler
	myHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, authenticated user!")
	})

	// Create server with basic auth middleware
	config := diohttp.Config{ListenAddress: ":8080"}
	server := diohttp.NewServer(config)
	server.AddHandler("/protected", authHandler.Wrap(myHandler))

	fmt.Println("Server configured with basic authentication")
	// Output: Server configured with basic authentication
}
