# Basic HTTP Server with Authentication

This example demonstrates how to create an HTTP server with basic authentication.

## Running the Example

```bash
go run github.com/dioad/net/examples/basic-http-server
```

Or build and run:
```bash
cd examples/basic-http-server
go build
./basic-http-server
```

Then test with:
```bash
curl -u user1:password1 http://localhost:8080/protected
```

## What It Demonstrates

- Creating a basic authentication map with username/password
- Setting up an HTTP server with basic auth middleware
- Protecting specific routes with authentication
- Graceful shutdown handling

## Code

See [main.go](main.go) for the complete executable example.
