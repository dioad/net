package prefixlist_test

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/dioad/net/authz/prefixlist"
	"github.com/rs/zerolog"
)

// Example demonstrates basic usage of the prefix list system
func Example() {
	logger := zerolog.Nop()

	// Create a multi-provider with GitLab provider
	gitlabProvider := prefixlist.NewGitLabProvider()
	multiProvider := prefixlist.NewMultiProvider([]prefixlist.Provider{gitlabProvider}, logger)

	ctx := context.Background()
	_, err := multiProvider.FetchPrefixes(ctx)
	if err != nil {
		panic(err)
	}

	// Check if an IP is in the allowed list
	addr, err := netip.ParseAddr("34.74.90.65")
	if err != nil {
		panic(err)
	}
	if multiProvider.Contains(addr) {
		fmt.Println("IP is allowed")
	} else {
		fmt.Println("IP is denied")
	}

	// Output: IP is allowed
}

// ExampleNewMultiProviderFromConfig demonstrates creating a multi-provider from configuration
func ExampleNewMultiProviderFromConfig() {
	logger := zerolog.Nop()

	config := prefixlist.Config{
		Providers: []prefixlist.ProviderConfig{
			{
				Name:    "github",
				Enabled: true,
				Filter:  map[string]string{"service": "hooks"}, // Only GitHub webhook IPs
			},
			{
				Name:    "gitlab",
				Enabled: true,
			},
		},
	}

	multiProvider, err := prefixlist.NewMultiProviderFromConfig(config, logger)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	_, err = multiProvider.FetchPrefixes(ctx)
	if err != nil {
		panic(err)
	}

	fmt.Println("MultiProvider created with multiple providers")
	// Output: MultiProvider created with multiple providers
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

	// Create a multi-provider with GitLab provider
	gitlabProvider := prefixlist.NewGitLabProvider()
	multiProvider := prefixlist.NewMultiProvider([]prefixlist.Provider{gitlabProvider}, logger)

	ctx := context.Background()
	_, err = multiProvider.FetchPrefixes(ctx)
	if err != nil {
		panic(err)
	}

	// Wrap with prefix list listener
	plListener := prefixlist.NewListener(baseListener, multiProvider, logger)

	fmt.Printf("Listening on %s with prefix list filtering\n", plListener.Addr())

	// Now only connections from allowed IPs will be accepted
	// conn, err := plListener.Accept()
}
