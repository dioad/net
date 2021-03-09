package auth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v33/github"
	"github.com/pkg/errors"
)

// only need ClientID for device flow
type GitHubAuthCommonConfig struct {
	ClientID     string `mapstructure:"client-id"`
	ClientSecret string `mapstructure:"client-secret"`

	// HTPasswdFile containing ClientID and ClientSecret
	ConfigFile string `mapstructure:"config-file"`
}

type GitHubAuthClientConfig struct {
	GitHubAuthCommonConfig `mapstructure:",squash"`
	AccessToken            string `mapstructure:"access-token"`
	AccessTokenFile        string `mapstructure:"access-token-file"`
}

func loadAccessTokenFromFile(filePath string) (string, error) {
	return "", nil
}

func resolveAccessToken(c GitHubAuthClientConfig) (string, error) {
	if c.AccessToken == "" {
		// load from c.AccessTokenFile
		token, err := loadAccessTokenFromFile(c.AccessTokenFile)
		if err != nil {
			return "", err
		}
		return token, nil
	}

	return c.AccessToken, nil
}

type GitHubClientAuth struct {
	Config      GitHubAuthClientConfig
	AccessToken string
}

func (a GitHubClientAuth) AddAuth(req *http.Request) error {
	if a.AccessToken == "" {
		var err error
		a.AccessToken, err = resolveAccessToken(a.Config)
		if err != nil {
			return err
		}
	}

	req.Header.Add("Authorization", fmt.Sprintf("bearer %v", a.AccessToken))

	return nil
}

type GitHubAuthServerConfig struct {
	GitHubAuthCommonConfig `mapstructure:",squash"`
}

type GitHubAuthenticator struct {
	Config GitHubAuthServerConfig
	Client *github.Client
}

func (a *GitHubAuthenticator) AuthenticateToken(accessToken string) (*github.User, error) {
	authorization, response, err := a.Client.Authorizations.Check(context.Background(), a.Config.ClientID, accessToken)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, errors.New(response.Status)
	}

	return authorization.User, nil
}

func NewGitHubAuthenticator(cfg GitHubAuthServerConfig) *GitHubAuthenticator {
	basicAuthTransport := github.BasicAuthTransport{
		Username: cfg.ClientID,
		Password: cfg.ClientSecret,
	}

	return &GitHubAuthenticator{
		Config: cfg,
		Client: github.NewClient(basicAuthTransport.Client()),
	}
}
