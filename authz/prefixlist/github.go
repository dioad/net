package prefixlist

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// GitHubProvider fetches IP ranges from GitHub's meta API
type GitHubProvider struct {
	filter string // "hooks", "git", "actions", "pages", "importer", "dependabot", or empty for all
}

type githubMeta struct {
	Hooks      []string `json:"hooks"`
	Git        []string `json:"git"`
	Actions    []string `json:"actions"`
	Pages      []string `json:"pages"`
	Importer   []string `json:"importer"`
	Dependabot []string `json:"dependabot"`
}

// NewGitHubProvider creates a new GitHub prefix list provider
func NewGitHubProvider(filter string) *GitHubProvider {
	return &GitHubProvider{
		filter: filter,
	}
}

func (p *GitHubProvider) Name() string {
	if p.filter != "" {
		return fmt.Sprintf("github-%s", p.filter)
	}
	return "github"
}

func (p *GitHubProvider) CacheDuration() time.Duration {
	return 1 * time.Hour
}

func (p *GitHubProvider) FetchPrefixes(ctx context.Context) ([]*net.IPNet, error) {
	body, err := fetchURL(ctx, "https://api.github.com/meta")
	if err != nil {
		return nil, err
	}

	var meta githubMeta
	if err := json.Unmarshal(body, &meta); err != nil {
		return nil, err
	}

	// Collect CIDR ranges based on filter
	var cidrs []string
	switch p.filter {
	case "hooks":
		cidrs = meta.Hooks
	case "git":
		cidrs = meta.Git
	case "actions":
		cidrs = meta.Actions
	case "pages":
		cidrs = meta.Pages
	case "importer":
		cidrs = meta.Importer
	case "dependabot":
		cidrs = meta.Dependabot
	default:
		// Include all
		cidrs = append(cidrs, meta.Hooks...)
		cidrs = append(cidrs, meta.Git...)
		cidrs = append(cidrs, meta.Actions...)
		cidrs = append(cidrs, meta.Pages...)
		cidrs = append(cidrs, meta.Importer...)
		cidrs = append(cidrs, meta.Dependabot...)
	}

	return parseCIDRs(cidrs)
}
