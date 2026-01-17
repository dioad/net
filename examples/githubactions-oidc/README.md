# GitHub Actions OIDC Example

This example demonstrates how to use GitHub Actions OIDC for authentication.

## Client Example (Getting OIDC Token)

```go
package main

import (
    "fmt"
    "os"
    
    "github.com/dioad/net/oidc/githubactions"
)

func main() {
    // Create a token source with optional audience
    tokenSource := githubactions.NewTokenSource(
        githubactions.WithAudience("https://github.com/dioad"),
    )
    
    // Get an OIDC token
    token, err := tokenSource.Token()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to get token: %v\n", err)
        os.Exit(1)
    }
    
    fmt.Printf("Token: %s\n", token.AccessToken)
    fmt.Printf("Expiry: %s\n", token.Expiry)
}
```

## Server Example (Validating OIDC Token)

```go
package main

import (
    "context"
    "fmt"
    "os"
    
    "github.com/dioad/net/oidc"
)

func main() {
    ctx := context.Background()
    
    // Create a validator config
    validatorConfig := oidc.ValidatorConfig{
        EndpointConfig: oidc.EndpointConfig{
            Type: "githubactions",
            URL:  "https://token.actions.githubusercontent.com",
        },
        Audiences: []string{"https://github.com/dioad"},
        Issuer:    "https://token.actions.githubusercontent.com",
    }
    
    // Create a validator
    validator, err := oidc.NewValidatorFromConfig(&validatorConfig)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to create validator: %v\n", err)
        os.Exit(1)
    }
    
    // Validate a token (example token - replace with actual token)
    tokenString := os.Getenv("GITHUB_ACTIONS_TOKEN")
    claims, err := validator.ValidateToken(ctx, tokenString)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to validate token: %v\n", err)
        os.Exit(1)
    }
    
    fmt.Printf("Claims: %+v\n", claims)
}
```

## Configuration Examples

### Client Configuration (YAML)

For configuring a client to use GitHub Actions OIDC:

```yaml
identity:
  type: githubactions
  audience: "https://github.com/dioad"
```

### Server Configuration (YAML)

For configuring a server to validate GitHub Actions OIDC tokens:

```yaml
jwt-validators:
  - type: githubactions
    url: "https://token.actions.githubusercontent.com"
    audiences:
      - "https://github.com/dioad"
    issuer: "https://token.actions.githubusercontent.com"
```

## GitHub Actions Workflow Setup

To use OIDC in GitHub Actions, you need to grant the workflow `id-token: write` permission:

```yaml
name: Example Workflow

on:
  push:
    branches: [ main ]

permissions:
  id-token: write
  contents: read

jobs:
  example:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Use OIDC Token
        run: |
          go run main.go
        env:
          OIDC_AUDIENCE: "https://github.com/dioad"
```

## GitHub Actions Claims

The GitHub Actions OIDC token includes the following claims:

- `iss`: Token issuer (https://token.actions.githubusercontent.com)
- `sub`: Subject (e.g., repo:owner/repo:ref:refs/heads/main)
- `aud`: Audience (customizable)
- `actor`: GitHub username that triggered the workflow
- `repository`: Repository name (owner/repo)
- `repository_owner`: Repository owner
- `workflow`: Workflow name
- `ref`: Git ref that triggered the workflow
- `sha`: Commit SHA
- And many more...

For a complete list of claims, see the [GitHub documentation](https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/about-security-hardening-with-openid-connect#understanding-the-oidc-token).
