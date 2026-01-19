# Network Rate Limiting

This example demonstrates network-level rate limiting by source IP address.

## Running the Example

```bash
go test -v github.com/dioad/net/examples/rate-limiting-network
```

## What It Demonstrates

- Creating a generic rate limiter for network connections
- Wrapping a listener with rate limiting
- Rate limiting by source IP address

## Code

See [example_test.go](example_test.go) for the complete executable example.
