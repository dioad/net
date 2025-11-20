package prefixlist_test

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/dioad/net/authz/prefixlist"
	"github.com/rs/zerolog"
)

// Example demonstrates basic usage of the prefix list system
func Example() {
	logger := zerolog.Nop()

	// Create a manager with GitLab provider
	gitlabProvider := prefixlist.NewGitLabProvider()
	manager := prefixlist.NewManager([]prefixlist.Provider{gitlabProvider}, logger)

	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		panic(err)
	}
	defer manager.Stop()

	// Check if an IP is in the allowed list
	ip := net.ParseIP("34.74.90.65")
	if manager.Contains(ip) {
		fmt.Println("IP is allowed")
	} else {
		fmt.Println("IP is denied")
	}

	// Output: IP is allowed
}

// ExampleNewManagerFromConfig demonstrates creating a manager from configuration
func ExampleNewManagerFromConfig() {
	logger := zerolog.Nop()

	config := prefixlist.Config{
		Providers: []prefixlist.ProviderConfig{
			{
				Name:    "github",
				Enabled: true,
				Filter:  "hooks", // Only GitHub webhook IPs
			},
			{
				Name:    "gitlab",
				Enabled: true,
			},
		},
		UpdateInterval: 1 * time.Hour,
	}

	manager, err := prefixlist.NewManagerFromConfig(config, logger)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		panic(err)
	}
	defer manager.Stop()

	fmt.Println("Manager started with multiple providers")
	// Output: Manager started with multiple providers
}

// ExampleListener demonstrates using the prefix list listener
func ExampleListener() {
	logger := zerolog.Nop()

	// Create a base listener
	baseListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer baseListener.Close()

	// Create a manager with GitLab provider
	gitlabProvider := prefixlist.NewGitLabProvider()
	manager := prefixlist.NewManager([]prefixlist.Provider{gitlabProvider}, logger)

	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		panic(err)
	}
	defer manager.Stop()

	// Wrap with prefix list listener
	plListener := prefixlist.NewListener(baseListener, manager, logger)

	fmt.Printf("Listening on %s with prefix list filtering\n", plListener.Addr())

	// Now only connections from allowed IPs will be accepted
	// conn, err := plListener.Accept()
}
