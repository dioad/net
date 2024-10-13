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

func ResolveToken(c ClientConfig) (*oauth2.Token, error) {
	if c.TokenFile != "" {
		token, err := LoadTokenFromFile(c.TokenFile)
		if err != nil {
			return nil, err
		}
		return token, nil
	}

	return nil, fmt.Errorf("no token found")
}
