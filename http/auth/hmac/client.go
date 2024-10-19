package hmac

import (
	"fmt"
	"net/http"
)

type ClientAuth struct {
	Config ClientConfig
}

// AddAuth adds the HMAC token to the request as a bearer token
//
// TODO:  This should be refactored to use the request Body to calculate the digest /token
func (a ClientAuth) AddAuth(req *http.Request) error {
	token, err := HMACKey([]byte(a.Config.SharedKey), []byte(a.Config.Data))
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", fmt.Sprintf("bearer %v", token))

	return nil
}

func (a ClientAuth) HTTPClient() (*http.Client, error) {
	token, err := HMACKey([]byte(a.Config.SharedKey), []byte(a.Config.Data))
	if err != nil {
		return nil, err
	}

	return &http.Client{
		Transport: &BearerTokenRoundTripper{
			BearerToken: token,
		},
	}, nil
}

type BearerTokenRoundTripper struct {
	BearerToken string
	Base        http.RoundTripper
}

func (t *BearerTokenRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.BearerToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.BearerToken))
	}

	if t.Base == nil {
		return http.DefaultTransport.RoundTrip(req)
	}

	return t.Base.RoundTrip(req)
}
