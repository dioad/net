# Prefix List ACL

The `prefixlist` package provides dynamic IP prefix list management for network access control. It allows you to easily restrict connections to trusted IP ranges from various cloud providers and services.

## Features

- **Multiple Provider Support**: Built-in support for GitHub, Cloudflare, Google Cloud, Atlassian, GitLab, and AWS
- **Automatic Updates**: Periodically fetches and caches IP ranges from providers
- **Listener Pattern**: Easy integration using the familiar `net.Listener` interface
- **YAML Configuration**: Simple configuration with YAML tags for easy integration
- **Efficient Matching**: Smart CIDR-based IP matching
- **Extensible**: Easy to add custom providers

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

## Usage

### Basic Usage with Code

```go
import (
    "context"
    "github.com/dioad/net/authz/prefixlist"
    "github.com/rs/zerolog"
)

// Create a manager with GitHub webhook IPs
logger := zerolog.New(os.Stdout)
provider := prefixlist.NewGitHubProvider("hooks")
manager := prefixlist.NewManager([]prefixlist.Provider{provider}, logger)

ctx := context.Background()
if err := manager.Start(ctx); err != nil {
    log.Fatal(err)
}
defer manager.Stop()

// Check if an IP is allowed
ip := net.ParseIP("192.30.252.1")
if manager.Contains(ip) {
    fmt.Println("Connection allowed")
}
```

### Using Configuration

```go
config := prefixlist.Config{
    Providers: []prefixlist.ProviderConfig{
        {
            Name:    "github",
            Enabled: true,
            Filter:  "hooks",
        },
        {
            Name:    "cloudflare",
            Enabled: true,
        },
    },
}

manager, err := prefixlist.NewManagerFromConfig(config, logger)
if err != nil {
    log.Fatal(err)
}
```

### YAML Configuration

```yaml
prefixlist:
  update_interval: 1h
  providers:
    - name: github
      enabled: true
      filter: hooks
    - name: cloudflare
      enabled: true
    - name: aws
      enabled: true
      filter: "EC2:us-east-1"
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

The GitHub provider supports filtering by service type:
- `hooks`: GitHub webhooks
- `git`: Git protocol access
- `actions`: GitHub Actions runners
- `pages`: GitHub Pages
- `importer`: GitHub Importer
- `dependabot`: Dependabot
- Empty string (default): All GitHub services

```go
// Only GitHub webhooks
provider := prefixlist.NewGitHubProvider("hooks")

// All GitHub services
provider := prefixlist.NewGitHubProvider("")
```

### Cloudflare

The Cloudflare provider supports IPv4 and IPv6:

```go
// IPv4 ranges
provider := prefixlist.NewCloudflareProvider(false)

// IPv6 ranges
provider := prefixlist.NewCloudflareProvider(true)
```

Or via config:
```yaml
- name: cloudflare
  enabled: true
  filter: ipv6  # or omit for IPv4
```

### AWS

The AWS provider supports filtering by service and region:

```go
// All AWS services in all regions
provider := prefixlist.NewAWSProvider("", "")

// Only EC2 in all regions
provider := prefixlist.NewAWSProvider("EC2", "")

// Only EC2 in us-east-1
provider := prefixlist.NewAWSProvider("EC2", "us-east-1")
```

Or via config:
```yaml
- name: aws
  enabled: true
  filter: "EC2:us-east-1"  # service:region format
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
