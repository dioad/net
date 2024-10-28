package oidc

import (
	"fmt"

	"golang.org/x/oauth2"

	"github.com/dioad/util"
)

func LoadTokenFromFile(filePath string) (*oauth2.Token, error) {
	return util.LoadStructFromFile[oauth2.Token](filePath)
}

func SaveTokenToFile(accessToken *oauth2.Token, authFilePath string) error {
	return util.SaveStructToFile[oauth2.Token](accessToken, authFilePath)
}

func ResolveTokenFromFile(tokenFile string) (*oauth2.Token, error) {
	if tokenFile != "" {
		token, err := LoadTokenFromFile(tokenFile)
		if err != nil {
			return nil, err
		}
		return token, nil
	}

	return nil, fmt.Errorf("no token found")
}
