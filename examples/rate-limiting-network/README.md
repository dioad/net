# Network Rate Limiting

This example demonstrates network-level rate limiting by source IP address.

## Running the Example

```bash
go run github.com/dioad/net/examples/rate-limiting-network
```

Or build and run:
```bash
cd examples/rate-limiting-network
go build
./rate-limiting-network
```

Then test with a TCP client:
```bash
nc localhost 8080
# or
telnet localhost 8080
```

## What It Demonstrates

- Creating a generic rate limiter for network connections
- Wrapping a listener with rate limiting
- Rate limiting by source IP address
- Interactive TCP echo server with rate limiting
- Graceful shutdown handling

## Code

See [main.go](main.go) for the complete executable example.
