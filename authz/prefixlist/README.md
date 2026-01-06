# Prefix List ACL

The `prefixlist` package provides dynamic IP prefix list management for network access control. It allows you to easily restrict connections to trusted IP ranges from various cloud providers and services.

## Features

- **Multiple Provider Support**: Built-in support for GitHub, Cloudflare, Google Cloud, Atlassian, GitLab, AWS, Fastly, and Hetzner
- **Self-contained Caching**: Each provider manages its own cache with stale-while-revalidate support
- **Listener Pattern**: Easy integration using the familiar `net.Listener` interface
- **YAML Configuration**: Simple configuration with YAML tags for easy integration
- **Modern IP Handling**: Uses `net/netip.Prefix` and `net/netip.Addr` for efficient IP operations
- **Extensible Architecture**: Generic HTTP providers for JSON and text-based endpoints
- **Idiomatic Go**: Clean, composable design following Go best practices

## Supported Providers

| Provider | Description | Default Cache Duration |
|----------|-------------|------------------------|
| GitHub | GitHub services (webhooks, actions, git, etc.) | 1 hour |
| Cloudflare | Cloudflare CDN (IPv4 and IPv6) | 24 hours |
| Google | Google Cloud Platform | 24 hours |
| Atlassian | Atlassian Cloud services | 24 hours |
| GitLab | GitLab webhooks (static IPs) | 7 days |
| AWS | Amazon Web Services (with optional service/region filtering) | 24 hours |
| Fastly | Fastly CDN | 24 hours |
| Hetzner | Hetzner Cloud (static ranges) | 7 days |

## Architecture

The package uses a clean, composable architecture:

- **`Provider` interface**: All providers implement `Prefixes(ctx) ([]netip.Prefix, error)` and `Contains(netip.Addr) bool`
- **`HTTPJSONProvider[T]`**: Generic provider for JSON-based HTTP endpoints with custom transform functions
- **`HTTPTextProvider`**: Provider for plain text CIDR lists (e.g., Cloudflare)
- **`CachingFetcher[T]`**: Generic caching layer with stale-while-revalidate support
- **`MultiProvider`**: Combines multiple providers into one (also implements `Provider` interface)

This design makes it easy to:
- Add new providers by creating simple transform functions
- Compose providers (MultiProvider accepts any Provider)
- Use providers standalone or in combination
- Support both JSON and text-based HTTP endpoints

## Usage

### Basic Usage with Code

```go
import (
    "context"
    "github.com/dioad/net/authz/prefixlist"
    "github.com/rs/zerolog"
)

// Create a provider for GitHub webhook IPs
logger := zerolog.New(os.Stdout)
provider := prefixlist.NewGitHubProvider("hooks")

// Use the provider to check IPs
ctx := context.Background()
prefixes, err := provider.Prefixes(ctx)
if err != nil {
    log.Fatal(err)
}

// Or check if an IP is allowed
addr := netip.MustParseAddr("192.30.252.1")
if provider.Contains(addr) {
    fmt.Println("Connection allowed")
}
```

### Using MultiProvider with Configuration

The map-based filter format provides clear, explicit filtering:

```go
config := prefixlist.Config{
    Providers: []prefixlist.ProviderConfig{
        {
            Name:    "github",
            Enabled: true,
            Filter:  map[string]string{"service": "hooks"},
        },
        {
            Name:    "aws",
            Enabled: true,
            Filter:  map[string]string{"service": "EC2", "region": "us-east-1"},
        },
        {
            Name:    "google",
            Enabled: true,
            Filter:  map[string]string{"scope": "us-central1", "service": "Google Cloud"},
        },
    },
}

multiProvider, err := prefixlist.NewMultiProviderFromConfig(config, logger)
if err != nil {
    log.Fatal(err)
}
```

### YAML Configuration

Use explicit key-value pairs for clarity:

```yaml
prefixlist:
  update_interval: 1h
  providers:
    - name: github
      enabled: true
      filter:
        service: hooks
    - name: aws
      enabled: true
      filter:
        service: EC2
        region: us-east-1
    - name: google
      enabled: true
      filter:
        scope: us-central1
        service: Google Cloud
    - name: atlassian
      enabled: true
      filter:
        region: global
        product: jira
```

### Using with net.Listener

```go
// Create base listener
baseListener, err := net.Listen("tcp", ":8080")
if err != nil {
    log.Fatal(err)
}

// Wrap with prefix list filtering
plListener := prefixlist.NewListener(baseListener, manager, logger)

// Accept connections (only from allowed IPs)
for {
    conn, err := plListener.Accept()
    if err != nil {
        log.Fatal(err)
    }
    go handleConnection(conn)
}
```

## Provider-Specific Options

### GitHub

The GitHub provider supports filtering by service type using the `service` key:

**Available services**: `hooks`, `git`, `actions`, `pages`, `importer`, `dependabot`

```go
// Only GitHub webhooks
provider := prefixlist.NewGitHubProvider("hooks")

// All GitHub services
provider := prefixlist.NewGitHubProvider("")
```

**YAML Configuration**:
```yaml
- name: github
  enabled: true
  filter:
    service: hooks  # or actions, git, pages, importer, dependabot
```

### Cloudflare

The Cloudflare provider supports IPv4 and IPv6 using the `version` key:

```go
// IPv4 ranges
provider := prefixlist.NewCloudflareProvider(false)

// IPv6 ranges
provider := prefixlist.NewCloudflareProvider(true)
```

**YAML Configuration**:
```yaml
- name: cloudflare
  enabled: true
  filter:
    version: ipv6  # or omit for IPv4
```

### Google Cloud

The Google Cloud provider supports filtering by scope (region) and service:

```go
// All Google Cloud IP ranges
provider := prefixlist.NewGoogleProvider(nil, nil)

// Only specific regions
provider := prefixlist.NewGoogleProvider([]string{"us-central1", "europe-west1"}, nil)

// Only specific services
provider := prefixlist.NewGoogleProvider(nil, []string{"Google Cloud"})

// Specific regions and services
provider := prefixlist.NewGoogleProvider(
    []string{"us-central1"}, 
    []string{"Google Cloud", "Google Cloud Storage"},
)
```

**YAML Configuration** (supports comma-separated values):
```yaml
- name: google
  enabled: true
  filter:
    scope: us-central1,europe-west1    # comma-separated regions
    service: Google Cloud               # single or comma-separated services
```

### Atlassian

The Atlassian provider supports filtering by `region` and `product` keys. **Note: Only prefixes with "egress" direction are included.**

```go
// All Atlassian IP ranges (egress only)
provider := prefixlist.NewAtlassianProvider(nil, nil)

// Only specific regions
provider := prefixlist.NewAtlassianProvider([]string{"global", "us-east-1"}, nil)

// Only specific products
provider := prefixlist.NewAtlassianProvider(nil, []string{"jira", "confluence"})

// Specific regions and products
provider := prefixlist.NewAtlassianProvider(
    []string{"global"}, 
    []string{"jira", "confluence"},
)
```

**YAML Configuration** (supports comma-separated values):
```yaml
- name: atlassian
  enabled: true
  filter:
    region: global,us-east-1           # comma-separated regions
    product: jira,confluence            # comma-separated products
```

### AWS

The AWS provider supports filtering by `service` and `region` keys:

```go
// All AWS services in all regions
provider := prefixlist.NewAWSProvider("", "")

// Only EC2 in all regions
provider := prefixlist.NewAWSProvider("EC2", "")

// Only EC2 in us-east-1
provider := prefixlist.NewAWSProvider("EC2", "us-east-1")
```

**YAML Configuration**:
```yaml
- name: aws
  enabled: true
  filter:
    service: EC2                        # specific AWS service
    region: us-east-1                   # specific AWS region
```

### GitLab

The GitLab provider uses static IP ranges for webhooks:
- `34.74.90.64/28`
- `34.74.226.0/24`

Note: GitLab Actions run on Google Cloud Platform, so enable the Google provider if you need to allow GitLab Actions runners.

### Fastly

The Fastly provider fetches IP ranges from Fastly's public API.

```go
provider := prefixlist.NewFastlyProvider()
```

### Hetzner

The Hetzner provider uses static IP ranges for Hetzner Cloud services. This includes all major Hetzner Cloud data centers.

```go
provider := prefixlist.NewHetznerProvider()
```

## Integration with Existing authz Package

The prefix list system works alongside the existing `authz.NetworkACL` and `authz.Listener`:

```go
// Combine static ACL with dynamic prefix lists
staticACL, _ := authz.NewNetworkACL(authz.NetworkACLConfig{
    AllowedNets: []string{"10.0.0.0/8"},
})

// You can use both:
// 1. authz.Listener for static allow/deny lists
staticListener := &authz.Listener{
    NetworkACL: staticACL,
    Listener:   baseListener,
    Logger:     logger,
}

// 2. prefixlist.Listener for dynamic provider-based lists
plListener := prefixlist.NewListener(staticListener, manager, logger)
```

## Custom Providers

To add a custom provider, implement the `Provider` interface:

```go
type CustomProvider struct{}

func (p *CustomProvider) Name() string {
    return "custom"
}

func (p *CustomProvider) CacheDuration() time.Duration {
    return 1 * time.Hour
}

func (p *CustomProvider) FetchPrefixes(ctx context.Context) ([]*net.IPNet, error) {
    // Fetch and parse your IP ranges
    cidrs := []string{"203.0.113.0/24"}
    return parseCIDRs(cidrs)
}
```

## Performance Considerations

- Prefix lists are cached in memory and updated periodically
- IP matching uses efficient CIDR comparison
- Updates happen in the background without blocking connections
- Failed updates retain previously cached data

## Security Notes

- The system gracefully handles provider failures by retaining cached data
- If all providers fail on initial start, an error is returned
- Connections are rejected during startup until at least one provider succeeds
- All HTTP requests to providers have a 30-second timeout

## Creating Custom Providers

The package provides generic base types for easy provider creation:

### JSON-based HTTP Provider

For endpoints that return JSON:

```go
import (
    "net/netip"
    "time"
    "github.com/dioad/net/authz/prefixlist"
)

type myServiceData struct {
    IPRanges []string `json:"ip_ranges"`
}

func transformMyService(data myServiceData) ([]netip.Prefix, error) {
    return prefixlist.ParseCIDRs(data.IPRanges)
}

provider := prefixlist.NewHTTPJSONProvider[myServiceData](
    "myservice",
    "https://api.myservice.com/ip-ranges",
    prefixlist.CacheConfig{
        StaticExpiry: 24 * time.Hour,
        ReturnStale:  true,
    },
    transformMyService,
)
```

### Text-based HTTP Provider

For endpoints that return plain text CIDR lists:

```go
provider := prefixlist.NewHTTPTextProvider(
    "myservice",
    "https://myservice.com/ips.txt",
    prefixlist.CacheConfig{
        StaticExpiry: 24 * time.Hour,
        ReturnStale:  true,
    },
)
```

### Static Provider

For static IP lists:

```go
type StaticProvider struct {
    name     string
    prefixes []netip.Prefix
}

func NewStaticProvider(name string, cidrs []string) (*StaticProvider, error) {
    prefixes, err := prefixlist.ParseCIDRs(cidrs)
    if err != nil {
        return nil, err
    }
    return &StaticProvider{name: name, prefixes: prefixes}, nil
}

func (p *StaticProvider) Name() string {
    return p.name
}

func (p *StaticProvider) Prefixes(ctx context.Context) ([]netip.Prefix, error) {
    return p.prefixes, nil
}

func (p *StaticProvider) Contains(addr netip.Addr) bool {
    for _, prefix := range p.prefixes {
        if prefix.Contains(addr) {
            return true
        }
    }
    return false
}
```

All providers implement the same `Provider` interface and can be used with `MultiProvider` or `Listener`.
