package prefixlist

import (
	"context"
	"net"
	"time"
)

// GitLabProvider provides static IP ranges for GitLab webhooks
type GitLabProvider struct{}

// NewGitLabProvider creates a new GitLab prefix list provider
func NewGitLabProvider() *GitLabProvider {
	return &GitLabProvider{}
}

func (p *GitLabProvider) Name() string {
	return "gitlab"
}

func (p *GitLabProvider) CacheDuration() time.Duration {
	// Static IPs, refresh less frequently
	return 7 * 24 * time.Hour
}

func (p *GitLabProvider) FetchPrefixes(ctx context.Context) ([]*net.IPNet, error) {
	// GitLab webhook static IPs
	// Note: GitLab Actions come from GCP, so users should also enable Google provider
	cidrs := []string{
		"34.74.90.64/28",
		"34.74.226.0/24",
	}

	return parseCIDRs(cidrs)
}
