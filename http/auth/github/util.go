package github

import (
	"os"

	"github.com/cli/oauth/api"

	"github.com/dioad/util"
)

func LoadAccessTokenFromFile(filePath string) (*api.AccessToken, error) {
	return util.LoadStructFromFile[api.AccessToken](filePath)
}

func SaveAccessTokenToFile(accessToken *api.AccessToken, authFilePath string) error {
	return util.SaveStructToFile[api.AccessToken](accessToken, authFilePath)
}

func ResolveAccessToken(c ClientConfig) (string, error) {

	if c.EnableAccessTokenFromEnvironment {
		if c.EnvironmentVariableName != "" {
			envAccessToken := os.Getenv(c.EnvironmentVariableName)

			if envAccessToken != "" {
				return envAccessToken, nil
			}
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
