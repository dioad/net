package github

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/cli/oauth/api"
	"github.com/rs/zerolog/log"

	//"github.com/dioad/cli/auth"
	"github.com/google/go-github/v33/github"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

func loadAccessTokenFromYamlFile(filePath string) (*api.AccessToken, error) {
	authFile, err := os.Open(filePath)
	if err != nil {
		log.Error().Str("filePath", filePath).Err(err).Msg("yamlAccessTokenFileError")
		fmt.Printf("error: %v", err)
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

type gitHubAuthenticator struct {
	Config GitHubAuthServerConfig
	Client *github.Client
}

func (a *gitHubAuthenticator) AuthenticateToken(accessToken string) (*github.User, error) {
	authorization, response, err := a.Client.Authorizations.Check(context.Background(), a.Config.ClientID, accessToken)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, errors.New(response.Status)
	}

	return authorization.User, nil
}

func NewGitHubAuthenticator(cfg GitHubAuthServerConfig) *gitHubAuthenticator {
	basicAuthTransport := github.BasicAuthTransport{
		Username: cfg.ClientID,
		Password: cfg.ClientSecret,
	}

	return &gitHubAuthenticator{
		Config: cfg,
		Client: github.NewClient(basicAuthTransport.Client()),
	}
}
