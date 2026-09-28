package prefixlist

import (
	"context"
	"net/netip"
)

func init() {
	RegisterProvider("gitlab", func(_ ProviderConfig) (Provider, error) {
		return NewGitLabProvider(), nil
	})
}

// GitLabProvider provides static IP ranges for GitLab webhooks.
type GitLabProvider struct {
	prefixes []netip.Prefix
}

// NewGitLabProvider creates a new GitLab prefix list provider.
func NewGitLabProvider() *GitLabProvider {
	// GitLab webhook static IPs
	// Note: GitLab Actions come from GCP, so users should also enable Google provider
	cidrs := []string{
		"34.74.90.64/28",
		"34.74.226.0/24",
	}

	prefixes, _ := parseCIDRs(cidrs) // Safe to ignore error as these are hard-coded valid CIDRs

	return &GitLabProvider{
		prefixes: prefixes,
	}
}

// Name returns the provider's name.
func (p *GitLabProvider) Name() string {
	return "gitlab"
}

// Prefixes returns the provider's static list of prefixes.
func (p *GitLabProvider) Prefixes(_ context.Context) ([]netip.Prefix, error) {
	return p.prefixes, nil
}

// Contains reports whether addr is contained in any of the provider's
// prefixes.
func (p *GitLabProvider) Contains(addr netip.Addr) bool {
	for _, prefix := range p.prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}
