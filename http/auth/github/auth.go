package github

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"

	"github.com/google/go-github/v33/github"
	"github.com/pkg/errors"
)

type Authenticator struct {
	Config GitHubAuthServerConfig
	Client *github.Client
}

type TokenSource struct {
	AccessToken string
}

func (t *TokenSource) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: t.AccessToken,
	}
	return token, nil
}

func (a *Authenticator) AuthenticateToken(accessToken string) (*UserInfo, error) {
	_, response, err := a.Client.Authorizations.Check(context.Background(), a.Config.ClientID, accessToken)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, errors.New(response.Status)
	}

	// get some info
	u, err := FetchUserInfo(accessToken)
	if err != nil {
		return nil, err
	}

	return u, nil
}

func NewGitHubAuthenticator(cfg GitHubAuthServerConfig) *Authenticator {
	basicAuthTransport := github.BasicAuthTransport{
		Username: cfg.ClientID,
		Password: cfg.ClientSecret,
	}

	return &Authenticator{
		Config: cfg,
		Client: github.NewClient(basicAuthTransport.Client()),
	}
}
