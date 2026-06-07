# github.com/dioad/net

A comprehensive Go library providing production-ready networking utilities, authorization, and security features for building robust networked applications.

Authentication and identity features are provided by the companion [`github.com/dioad/auth`](https://github.com/dioad/auth) library, which builds on top of this one.

## Overview

`dioad/net` is a feature-rich networking library that simplifies building secure, observable, and maintainable network services in Go.
It provides implementations of common networking patterns, network-level security, and infrastructure protocols.

## Core Features

### 🌐 HTTP Server
- **HTTP/HTTPS Server**: HTTP server based on `gorilla/mux` with TLS support
- **UNIX Socket Support**: Listen on UNIX domain sockets (via `Serve`)
- **Middleware Stack**: CORS, logging, metrics, header marshaling
- **Resource-based Routing**: Clean RESTful resource handlers
- **Proxy Protocol Support**: Load balancer integration via PROXY protocol
- **Metrics**: Built-in Prometheus metrics collection

### 🔒 TLS/Security
- **Certificate Management**: Generate, load, and validate X.509 certificates
- **Self-Signed Certificates**: Easy self-signed certificate generation for testing
- **Automatic Certificate Management**: ACME protocol support via Let's Encrypt (`autocert`)
- **Client Configuration**: Secure TLS client setup with custom verification
- **Server Configuration**: TLS server setup with certificate rotation

### 📧 SMTP/Email Security
- **Domain Security Records**: SPF, DKIM, DMARC, MTA-STS, TLS-RPT support
- **Email Authentication**: DKIM signing and verification utilities
- **DNS Record Generation**: Template-based record rendering for email security

### 🔍 DNS Utilities
- **DNS-over-HTTPS (DoH)**: Privacy-focused DNS resolution
- **IP Utilities**: IP address manipulation and validation
- **Blocklist Lookups**: Spam/blocklist checking via DNS (Spamhaus)

### 🛡️ Network Authorization
- **IP-based ACLs**: Network access control lists with allow/deny rules
- **Rate Limiting**: Per-principal rate limiting for network and HTTP services
- **Prefix Lists**: Support for cloud provider IP ranges (AWS, Google Cloud, Azure, Fastly, Cloudflare, Atlassian, GitLab, Hetzner)
- **Automatic Updates**: Background refresh of cloud provider prefix lists

### 📊 Metrics
- **Connection Metrics**: Track bytes sent/received per connection
- **Listener Metrics**: Network listener statistics
- **Prometheus Integration**: Native Prometheus metrics export

### 🔧 Connection Utilities
- **Connection Lifecycle**: Helpers for proper connection cleanup (`DoneConn`)
- **Context Integration**: Context-aware connection operations

## Quick Start

### Basic HTTP Server
```go
import (
	diohttp "github.com/dioad/net/http"
	"net/http"
)

myHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello!")
})

config := diohttp.Config{ListenAddress: ":8080"}
server := diohttp.NewServer(config)
server.AddHandler("/hello", myHandler)
server.ListenAndServe()
```

### HTTP Server with OIDC/JWT Authentication

Authentication middleware is provided by [`github.com/dioad/auth`](https://github.com/dioad/auth).

```go
import (
	diohttp "github.com/dioad/net/http"
	authserver "github.com/dioad/auth/http/server"
	"github.com/dioad/auth/oidc"
)

validatorConfig := oidc.ValidatorConfig{
	EndpointConfig: oidc.EndpointConfig{
		Type: "githubactions",
		URL:  "https://token.actions.githubusercontent.com",
	},
	Audiences: []string{"https://github.com/my-org"},
	Issuer:    "https://token.actions.githubusercontent.com",
}

config := diohttp.Config{ListenAddress: ":8080"}
server := diohttp.NewServer(config, authserver.WithOAuth2Validator([]oidc.ValidatorConfig{validatorConfig}))
server.AddHandler("/secure", myHandler)
```

### IP-based Access Control
```go
import (
	"github.com/dioad/net/authz"
)

// Create network ACL
acl, _ := authz.NewNetworkACL(authz.NetworkACLConfig{
	AllowedNets: []string{"10.0.0.0/8"},
	DeniedNets:  []string{"10.0.0.5"},
})

// Check if IP is authorised
if authorised, _ := acl.AuthoriseFromString(clientIP); authorised {
	// Allow access
}
```

### Rate Limiting (HTTP)
```go
import (
	"github.com/dioad/net/http"
	"github.com/rs/zerolog/log"
)

// Create rate limiter (1 request per second, burst of 5)
limiter := http.NewRateLimiter(1.0, 5, log.Logger)

// Use as middleware for a specific principal
handler := limiter.Middleware("user1")(myHandler)

// Or use middleware that extracts principal from context
// contextHandler := limiter.MiddlewareFromContext(authContextKey)(myHandler)
```

### Rate Limiting (Dynamic)
```go
import (
	"github.com/dioad/net/ratelimit"
	"github.com/dioad/net/http"
	"github.com/rs/zerolog/log"
)

// Implement RateLimitSource
type mySource struct{}
func (s *mySource) GetLimit(principal string) (float64, int, bool) {
	if principal == "premium" {
		return 100.0, 100, true
	}
	return 1.0, 5, true
}

// Create rate limiter with custom source
limiter := http.NewRateLimiterWithSource(&mySource{}, log.Logger)
```

### Rate Limiting (Network)
```go
import (
	"net"
	"github.com/dioad/net/ratelimit"
	"github.com/rs/zerolog/log"
)

// Create a generic rate limiter (10 connections per second, burst of 20)
rl := ratelimit.NewRateLimiter(10.0, 20, log.Logger)

// Wrap an existing listener with rate limiting (by source IP)
ln, _ := net.Listen("tcp", ":8080")
rlListener := ratelimit.NewListener(ln, rl, log.Logger)

// Use the rate-limited listener
// http.Serve(rlListener, myHandler)
```

### TLS Configuration
```go
import (
	"context"
	"github.com/dioad/net/http"
	"github.com/dioad/net/tls"
)

// Configure TLS
tlsServerConfig := tls.ServerConfig{
	CertFile: "/path/to/cert.pem",
	KeyFile:  "/path/to/key.pem",
}
tlsConfig, _ := tls.NewServerTLSConfig(context.Background(), tlsServerConfig)

// Create server with TLS
config := http.Config{
	ListenAddress: ":443",
	TLSConfig:     tlsConfig,
}
server := http.NewServer(config)
```

### More Examples

For more comprehensive, executable examples, see the [`examples/`](examples/) directory:

- **[Basic HTTP Server](examples/basic-http-server/)** - HTTP server setup and routing
- **[IP-based Access Control](examples/ip-acl/)** - Network ACLs for IP filtering
- **[HTTP Rate Limiting](examples/rate-limiting-http/)** - Per-principal HTTP rate limiting
- **[Dynamic Rate Limiting](examples/rate-limiting-dynamic/)** - Rate limiting with custom sources
- **[Network Rate Limiting](examples/rate-limiting-network/)** - Network-level rate limiting
- **[TLS Configuration](examples/tls-config/)** - TLS setup with self-signed certificates

Authentication examples (OIDC, JWT, GitHub Actions) are in [`github.com/dioad/auth/examples/`](https://github.com/dioad/auth/tree/main/examples).

All examples are standalone executable Go programs that can be run with `go run ./examples/...` or built with `go build ./examples/...`.

## Package Structure

- **`authz/`** - Authorization utilities (IP-based ACLs, principal allow/deny lists)
- **`dns/`** - DNS utilities (DoH, IP utilities, blocklist checks)
- **`http/`** - HTTP server and client
  - **`authz/`** - Authorization middleware (IP-based)
  - **`json/`** - JSON response helpers with structured logging
  - **`resource/`** - Resource-based request handlers
- **`ratelimit/`** - Generic per-principal rate limiting logic
- **`metrics/`** - Prometheus metrics collection
- **`smtp/`** - SMTP/email security (DKIM, DMARC, SPF, MTA-STS)
- **`tls/`** - TLS certificate management and ACME support

Authentication and identity (JWT, OIDC, basic auth, HMAC) are in [`github.com/dioad/auth`](https://github.com/dioad/auth).

## Requirements

- Go 1.24 or later
- Standard Go dependencies (see `go.mod`)

## License

See LICENSE file for details.
