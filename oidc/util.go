package oidc

import (
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"

	"github.com/dioad/util"
)

func LoadTokenFromFile(filePath string) (*oauth2.Token, error) {
	return util.LoadStructFromFile[oauth2.Token](filePath)
}

func SaveTokenToFile(accessToken *oauth2.Token, authFilePath string) error {
	return util.SaveStructToFile[oauth2.Token](accessToken, authFilePath)
}

var NoTokenFoundError = fmt.Errorf("no token found")

func tokenError(baseError error, err error) error {
	return fmt.Errorf("%w: %w", baseError, err)
}

func ResolveTokenFromFile(tokenFile string) (*oauth2.Token, error) {
	if tokenFile != "" {
		token, err := LoadTokenFromFile(tokenFile)
		if err != nil {
			return nil, tokenError(NoTokenFoundError, err)
		}
		return token, nil
	}

	return nil, NoTokenFoundError
}

func ExtractClaimsMap(accessToken string) (jwt.MapClaims, error) {
	parsedToken, _, err := new(jwt.Parser).ParseUnverified(accessToken, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("error parsing token: %w", err)
	}

	if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}
