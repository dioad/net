# Dynamic Rate Limiting

This example demonstrates dynamic rate limiting with custom sources that can provide different limits for different principals.

## Running the Example

```bash
go test -v github.com/dioad/net/examples/rate-limiting-dynamic
```

## What It Demonstrates

- Implementing a custom rate limit source
- Providing different rate limits for different principals (e.g., premium vs. standard users)
- Using dynamic rate limiting in HTTP middleware

## Code

See [example_test.go](example_test.go) for the complete executable example.
