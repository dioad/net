package auth

import (
	"context"
	"net/http"

	"github.com/google/go-github/v33/github"
	"github.com/pkg/errors"
)

type GitHubAuthConfig struct {
	ClientID     string `mapstructure:"client-id"`
	ClientSecret string `mapstructure:"client-secret"`
}

type GitHubAuthenticator struct {
	Config GitHubAuthConfig
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

func NewGitHubAuthenticator(cfg GitHubAuthConfig) *GitHubAuthenticator {
	basicAuthTransport := github.BasicAuthTransport{
		Username: cfg.ClientID,
		Password: cfg.ClientSecret,
	}

	return &GitHubAuthenticator{
		Config: cfg,
		Client: github.NewClient(basicAuthTransport.Client()),
	}
}
