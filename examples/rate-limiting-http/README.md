# HTTP Rate Limiting

This example demonstrates per-principal HTTP rate limiting.

## Running the Example

```bash
go run github.com/dioad/net/examples/rate-limiting-http
```

Or build and run:
```bash
cd examples/rate-limiting-http
go build
./rate-limiting-http
```

Then test by making multiple quick requests:
```bash
for i in {1..10}; do curl http://localhost:8080/limited; done
```

## What It Demonstrates

- Creating a rate limiter with requests per second and burst limits
- Using rate limiting as middleware for specific principals
- Protecting HTTP handlers with rate limits
- Observing rate limiting in action with rapid requests

## Code

See [main.go](main.go) for the complete executable example.
