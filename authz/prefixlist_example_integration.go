package authz

// This file demonstrates how to use the prefixlist package alongside
// the existing NetworkACL functionality.
//
// Example usage:
//
//	import (
//	    "context"
//	    "net"
//	    "github.com/dioad/net/authz"
//	    "github.com/dioad/net/authz/prefixlist"
//	    "github.com/rs/zerolog"
//	)
//
//	func createSecureListener() (net.Listener, error) {
//	    // Create base listener
//	    baseListener, err := net.Listen("tcp", ":8080")
//	    if err != nil {
//	        return nil, err
//	    }
//
//	    // Option 1: Use static NetworkACL for fixed allow/deny lists
//	    staticACL, err := authz.NewNetworkACL(authz.NetworkACLConfig{
//	        AllowedNets:    []string{"10.0.0.0/8"},
//	        DeniedNets:     []string{"10.1.0.0/16"},
//	        AllowByDefault: false,
//	    })
//	    if err != nil {
//	        return nil, err
//	    }
//
//	    staticListener := &authz.Listener{
//	        NetworkACL: staticACL,
//	        Listener:   baseListener,
//	        Logger:     logger,
//	    }
//
//	    // Option 2: Use dynamic prefix lists for cloud provider IPs
//	    config := prefixlist.Config{
//	        Providers: []prefixlist.ProviderConfig{
//	            {Name: "github", Enabled: true, Filter: "hooks"},
//	            {Name: "gitlab", Enabled: true},
//	        },
//	    }
//
//	    manager, err := prefixlist.NewManagerFromConfig(config, logger)
//	    if err != nil {
//	        return nil, err
//	    }
//
//	    ctx := context.Background()
//	    if err := manager.Start(ctx); err != nil {
//	        return nil, err
//	    }
//
//	    prefixListener := prefixlist.NewListener(baseListener, manager, logger)
//
//	    // Option 3: Combine both (static ACL + dynamic prefix lists)
//	    // First apply static ACL, then prefix list filtering
//	    combinedListener := prefixlist.NewListener(staticListener, manager, logger)
//
//	    return combinedListener, nil
//	}
