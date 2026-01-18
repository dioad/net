package prefixlist

import (
	"fmt"
	"net/netip"
	"time"
)

// GitHubProvider fetches IP ranges from GitHub's meta API
type GitHubProvider struct {
	*HTTPJSONProvider[githubMeta]
	filter string
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
	name := "github"
	if filter != "" {
		name = fmt.Sprintf("github-%s", filter)
	}

	p := &GitHubProvider{
		filter: filter,
	}

	p.HTTPJSONProvider = NewHTTPJSONProvider[githubMeta](
		name,
		"https://api.github.com/meta",
		CacheConfig{
			StaticExpiry: 1 * time.Hour,
			ReturnStale:  true,
		},
		p.transformGitHubMeta,
	)

	return p
}

func (p *GitHubProvider) transformGitHubMeta(meta githubMeta) ([]netip.Prefix, error) {
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
