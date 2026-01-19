package ipacl_test

import (
	"fmt"

	"github.com/dioad/net/authz"
)

// Example demonstrates creating and using network ACLs for IP-based access control.
func Example() {
	// Create network ACL
	acl, err := authz.NewNetworkACL(authz.NetworkACLConfig{
		AllowedNets: []string{"10.0.0.0/8"},
		DeniedNets:  []string{"10.0.0.5"},
	})
	if err != nil {
		fmt.Printf("Error creating ACL: %v\n", err)
		return
	}

	// Check if IP is authorised
	clientIP := "10.0.1.1:12345"
	if authorised, err := acl.AuthoriseFromString(clientIP); err != nil {
		fmt.Printf("Error checking authorization: %v\n", err)
	} else if authorised {
		fmt.Println("Access allowed")
	} else {
		fmt.Println("Access denied")
	}

	// Check denied IP
	deniedIP := "10.0.0.5:12345"
	if authorised, err := acl.AuthoriseFromString(deniedIP); err != nil {
		fmt.Printf("Error checking authorization: %v\n", err)
	} else if authorised {
		fmt.Println("Access allowed")
	} else {
		fmt.Println("Access denied for blocked IP")
	}
	// Output:
	// Access allowed
	// Access denied for blocked IP
}
