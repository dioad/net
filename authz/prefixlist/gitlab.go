package prefixlist

import (
	"context"
	"net/netip"
)

// GitLabProvider provides static IP ranges for GitLab webhooks
type GitLabProvider struct {
	prefixes []netip.Prefix
}

// NewGitLabProvider creates a new GitLab prefix list provider
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

func (p *GitLabProvider) Name() string {
	return "gitlab"
}

func (p *GitLabProvider) Prefixes(ctx context.Context) ([]netip.Prefix, error) {
	return p.prefixes, nil
}

func (p *GitLabProvider) Contains(addr netip.Addr) bool {
	for _, prefix := range p.prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
