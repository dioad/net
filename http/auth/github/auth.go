package github

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/cli/oauth/api"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"

	"github.com/google/go-github/v33/github"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

func loadAccessTokenFromYamlFile(filePath string) (*api.AccessToken, error) {
	filePath, err := homedir.Expand(filePath)
	if err != nil {
		return nil, err
	}

	authFile, err := os.Open(filePath)
	if err != nil {
		log.Error().Str("filePath", filePath).Err(err).Msg("yamlAccessTokenFileError")
		fmt.Printf("error: %v\n", err)
		return nil, err
	}
	defer authFile.Close()

	var accessToken api.AccessToken

	encoder := yaml.NewDecoder(authFile)
	encoder.Decode(&accessToken)

	return &accessToken, nil
}

func loadAccessTokenFromFile(filePath string) (*api.AccessToken, error) {
	if strings.HasSuffix(filePath, ".yaml") || strings.HasSuffix(filePath, ".yml") {
		log.Debug().Str("filePath", filePath).Msg("githubAccessTokenFile")
		return loadAccessTokenFromYamlFile(filePath)
	}
	return nil, errors.New("unrecognised access token file type. expect yaml")
}

func resolveAccessToken(c GitHubAuthClientConfig) (string, error) {
	if c.AccessToken == "" {
		// load from c.AccessTokenFile
		token, err := loadAccessTokenFromFile(c.AccessTokenFile)
		if err != nil {
			return "", err
		}
		return token.Token, nil
	}

	return c.AccessToken, nil
}

type authenticator struct {
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

func (a *authenticator) AuthenticateToken(accessToken string) (*UserInfo, error) {
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

func NewGitHubAuthenticator(cfg GitHubAuthServerConfig) *authenticator {
	basicAuthTransport := github.BasicAuthTransport{
		Username: cfg.ClientID,
		Password: cfg.ClientSecret,
	}

	return &authenticator{
		Config: cfg,
		Client: github.NewClient(basicAuthTransport.Client()),
	}
}
