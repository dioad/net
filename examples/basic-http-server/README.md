# Basic HTTP Server

This example is a minimal HTTP server built with `dioad/net/http`.

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
curl http://localhost:8080/hello
```

Stop it with Ctrl+C or `SIGTERM`. It exits with status 0 after a clean shutdown.

## What It Demonstrates

- Creating a server and adding a route
- `Start`, which returns a bind failure to the caller, and `Failed`, which reports a serve loop that ends unexpectedly
- Graceful shutdown on `SIGINT` or `SIGTERM`
- Structured logging with zerolog: a `service` field, and `zerolog.Ctx` in the handler

## Code

See [main.go](main.go) for the complete executable example.
