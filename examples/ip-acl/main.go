package main

import (
	"fmt"
	"log"

	"github.com/dioad/net/authz"
)

func main() {
	// Create network ACL
	acl, err := authz.NewNetworkACL(authz.NetworkACLConfig{
		AllowedNets: []string{"10.0.0.0/8"},
		DeniedNets:  []string{"10.0.0.5"},
	})
	if err != nil {
		log.Fatalf("Error creating ACL: %v\n", err)
	}

	fmt.Println("Network ACL created with:")
	fmt.Println("  Allowed: 10.0.0.0/8")
	fmt.Println("  Denied: 10.0.0.5")
	fmt.Println()

	// Check if IP is authorised
	clientIP := "10.0.1.1:12345"
	if authorised, err := acl.AuthoriseFromString(clientIP); err != nil {
		log.Printf("Error checking authorization: %v\n", err)
	} else if authorised {
		fmt.Printf("✓ %s - Access allowed\n", clientIP)
	} else {
		fmt.Printf("✗ %s - Access denied\n", clientIP)
	}

	// Check denied IP
	deniedIP := "10.0.0.5:12345"
	if authorised, err := acl.AuthoriseFromString(deniedIP); err != nil {
		log.Printf("Error checking authorization: %v\n", err)
	} else if authorised {
		fmt.Printf("✓ %s - Access allowed\n", deniedIP)
	} else {
		fmt.Printf("✗ %s - Access denied (explicitly blocked)\n", deniedIP)
	}

	// Check IP outside allowed range
	outsideIP := "192.168.1.1:12345"
	if authorised, err := acl.AuthoriseFromString(outsideIP); err != nil {
		log.Printf("Error checking authorization: %v\n", err)
	} else if authorised {
		fmt.Printf("✓ %s - Access allowed\n", outsideIP)
	} else {
		fmt.Printf("✗ %s - Access denied (outside allowed range)\n", outsideIP)
	}
}
