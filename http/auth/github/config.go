package github

var (
	EmptyGitHubAuthClientConfig = GitHubAuthClientConfig{}
	EmptyGitHubAuthServerConfig = GitHubAuthServerConfig{}
)

// only need ClientID for device flow
type GitHubAuthCommonConfig struct {
	ClientID     string `mapstructure:"client-id"`
	ClientSecret string `mapstructure:"client-secret"`

	// ConfigFile containing ClientID and ClientSecret
	ConfigFile string `mapstructure:"config-file"`
}

type GitHubAuthClientConfig struct {
	GitHubAuthCommonConfig `mapstructure:",squash"`
	AccessToken            string `mapstructure:"access-token"`
	AccessTokenFile        string `mapstructure:"access-token-file"`
}

type GitHubAuthServerConfig struct {
	GitHubAuthCommonConfig `mapstructure:",squash"`
	UserAllowList          []string `mapstructure:"user-allow-list"`
	UserDenyList           []string `mapstructure:"user-deny-list"`
}
