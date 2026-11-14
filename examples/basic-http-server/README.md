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
- A `*http.Server` run by `lifecycle.App`: no adapter, and a bind failure is an error from `Run`
- Graceful shutdown on `SIGINT` or `SIGTERM`, and an unexpected end of the serve loop ending the process
- Structured logging with zerolog: a `service` field, and `zerolog.Ctx` in the handler

## Code

See [main.go](main.go) for the complete executable example.
