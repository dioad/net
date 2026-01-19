# TLS Configuration

This example demonstrates configuring TLS for an HTTP server using self-signed certificates.

## Running the Example

```bash
go run github.com/dioad/net/examples/tls-config
```

Or build and run:
```bash
cd examples/tls-config
go build
./tls-config
```

Then test with curl:
```bash
curl -k https://localhost:8443/
```
(The `-k` flag skips certificate verification for self-signed certificates)

## What It Demonstrates

- Generating self-signed certificates programmatically
- Configuring TLS with local certificate files
- Setting up an HTTPS server with TLS
- Certificate subject and SAN configuration
- Graceful shutdown handling

## Code

See [main.go](main.go) for the complete executable example.
