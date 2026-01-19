# HTTP Rate Limiting

This example demonstrates per-principal HTTP rate limiting.

## Running the Example

```bash
go test -v github.com/dioad/net/examples/rate-limiting-http
```

## What It Demonstrates

- Creating a rate limiter with requests per second and burst limits
- Using rate limiting as middleware for specific principals
- Protecting HTTP handlers with rate limits

## Code

See [example_test.go](example_test.go) for the complete executable example.
