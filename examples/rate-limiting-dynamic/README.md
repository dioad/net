# Dynamic Rate Limiting

This example demonstrates dynamic rate limiting with custom sources that can provide different limits for different principals.

## Running the Example

```bash
go run github.com/dioad/net/examples/rate-limiting-dynamic
```

Or build and run:
```bash
cd examples/rate-limiting-dynamic
go build
./rate-limiting-dynamic
```

Then test the different endpoints:
```bash
# Test premium endpoint (higher limits)
for i in {1..10}; do curl http://localhost:8080/premium; done

# Test standard endpoint (lower limits)
for i in {1..10}; do curl http://localhost:8080/standard; done
```

## What It Demonstrates

- Implementing a custom rate limit source
- Providing different rate limits for different principals (e.g., premium vs. standard users)
- Using dynamic rate limiting in HTTP middleware
- Different rate limits for different endpoints

## Code

See [main.go](main.go) for the complete executable example.
