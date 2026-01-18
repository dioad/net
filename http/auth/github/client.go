package github

import (
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

type ClientAuth struct {
	Config      ClientConfig
	accessToken string
}

func (a ClientAuth) AddAuth(req *http.Request) error {
	if a.accessToken == "" {
		var err error
		a.accessToken, err = ResolveAccessToken(a.Config)
		if err != nil {
			return err
		}
		log.Debug().Str("accessTokenPrefix", a.accessToken[0:8]).Msg("readAccessToken")
	}

	req.Header.Add("Authorization", fmt.Sprintf("bearer %v", a.accessToken))

	return nil
}

func (a ClientAuth) Token() (*oauth2.Token, error) {
	if a.accessToken == "" {
		var err error
		a.accessToken, err = ResolveAccessToken(a.Config)
		if err != nil {
			return nil, err
		}
		log.Debug().Str("accessTokenPrefix", a.accessToken[0:8]).Msg("readAccessToken")
	}

	return &oauth2.Token{
		AccessToken: a.accessToken,
	}, nil
}

func (a ClientAuth) HTTPClient() *http.Client {
	return &http.Client{
		Transport: &TokenRoundTripper{
			Source: &a,
		},
	}
}

type TokenRoundTripper struct {
	Source oauth2.TokenSource
	Base   http.RoundTripper
}

func (t *TokenRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.Source != nil {

		token, err := t.Source.Token()
		if err != nil {
			return nil, err
		}
		token.SetAuthHeader(req)
	}

	if t.Base == nil {
		return http.DefaultTransport.RoundTrip(req)
	}

	return t.Base.RoundTrip(req)
}
