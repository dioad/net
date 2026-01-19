# IP-based Access Control

This example demonstrates how to use network ACLs for IP-based access control.

## Running the Example

```bash
go run github.com/dioad/net/examples/ip-acl
```

Or build and run:
```bash
cd examples/ip-acl
go build
./ip-acl
```

## What It Demonstrates

- Creating a network ACL with allowed and denied IP ranges
- Checking if specific IPs are authorized
- Using CIDR notation for network ranges
- Explicit IP blocking within allowed ranges

## Code

See [main.go](main.go) for the complete executable example.
