package github

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cli/oauth/api"
	"github.com/dioad/util"
	"gopkg.in/yaml.v3"
)

func ResolveAccessToken(c GitHubAuthClientConfig) (string, error) {

	if c.EnableAccessTokenFromEnvironment {
		envAccessToken := os.Getenv("GITHUB_TOKEN")

		if envAccessToken != "" {
			return envAccessToken, nil
		}
	}

	if c.AccessToken == "" {
		// load from c.AccessTokenFile
		token, err := LoadAccessTokenFromFile(c.AccessTokenFile)
		if err != nil {
			return "", err
		}
		return token.Token, nil
	}

	return c.AccessToken, nil
}
func loadAccessTokenFromYAMLReader(r io.Reader) (*api.AccessToken, error) {
	var accessToken api.AccessToken

	encoder := yaml.NewDecoder(r)
	err := encoder.Decode(&accessToken)
	if err != nil {
		return nil, err
	}

	return &accessToken, nil
}
func loadAccessTokenFromYAMLFile(filePath string) (*api.AccessToken, error) {
	authFile, err := util.CleanOpen(filePath)
	if err != nil {
		return nil, err
	}

	accessToken, err := loadAccessTokenFromYAMLReader(authFile)

	if err != nil {
		closeErr := authFile.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("%w: %v", err, closeErr)
		}
		return nil, err
	}

	return accessToken, authFile.Close()
}

func LoadAccessTokenFromFile(filePath string) (*api.AccessToken, error) {
	if strings.HasSuffix(filePath, ".yaml") || strings.HasSuffix(filePath, ".yml") {
		return loadAccessTokenFromYAMLFile(filePath)
	}
	return nil, errors.New("unrecognised access token file type. expect yaml")
}

func SaveAccessTokenToFile(accessToken *api.AccessToken, authFilePath string) error {
	authFile, err := util.CleanOpenFile(authFilePath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}

	err = saveAccessTokenToWriter(authFile, accessToken)
	if err != nil {
		closeErr := authFile.Close()
		if closeErr != nil {
			return fmt.Errorf("%w: %v", err, closeErr)
		}
		return err
	}

	return authFile.Close()
}

func saveAccessTokenToWriter(w io.Writer, accessToken *api.AccessToken) error {
	encoder := yaml.NewEncoder(w)
	return encoder.Encode(accessToken)
}
