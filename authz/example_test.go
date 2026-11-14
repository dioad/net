package authz_test

import (
	"errors"
	"fmt"

	"github.com/dioad/net/authz"
)

// A deny network wins over an allow network, and an address that matches
// neither is denied unless the ACL allows by default.
func ExampleNewNetworkACL() {
	acl, err := authz.NewNetworkACL(authz.NetworkACLConfig{
		AllowedNets: []string{"10.0.0.0/8"},
		DeniedNets:  []string{"10.0.0.5/32"},
	})
	if err != nil {
		fmt.Println("acl:", err)

		return
	}

	for _, addr := range []string{"10.0.1.1:443", "10.0.0.5:443", "192.168.1.1:443"} {
		err := acl.AuthoriseFromString(addr)
		fmt.Println(addr, "denied:", errors.Is(err, authz.ErrDenied))
	}
	// Output:
	// 10.0.1.1:443 denied: false
	// 10.0.0.5:443 denied: true
	// 192.168.1.1:443 denied: true
}
