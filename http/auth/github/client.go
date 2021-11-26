package github

import (
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
)

type GitHubClientAuth struct {
	Config      GitHubAuthClientConfig
	accessToken string
}

func (a GitHubClientAuth) AddAuth(req *http.Request) error {
	if a.accessToken == "" {
		var err error
		a.accessToken, err = resolveAccessToken(a.Config)
		if err != nil {
			return err
		}
		log.Debug().Str("accessTokenPrefix", a.accessToken[0:8]).Msg("readAccessToken")
	}

	req.Header.Add("Authorization", fmt.Sprintf("bearer %v", a.accessToken))

	return nil
}
