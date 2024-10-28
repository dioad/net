package flyio

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"golang.org/x/oauth2"

	http2 "github.com/dioad/net/http"
)

type Claims struct {
	AppId          string `json:"app_id"`
	AppName        string `json:"app_name"`
	Aud            string `json:"aud"`
	Exp            int    `json:"exp"`
	Iat            int    `json:"iat"`
	Image          string `json:"image"`
	ImageDigest    string `json:"image_digest"`
	Iss            string `json:"iss"`
	Jti            string `json:"jti"`
	MachineId      string `json:"machine_id"`
	MachineName    string `json:"machine_name"`
	MachineVersion string `json:"machine_version"`
	Nbf            int    `json:"nbf"`
	OrgId          string `json:"org_id"`
	OrgName        string `json:"org_name"`
	Region         string `json:"region"`
	Sub            string `json:"sub"`
}

type tokenSource struct {
	audience     string
	client       *http.Client
	currentToken *oauth2.Token
}

type Opt func(*tokenSource)

func WithAudience(aud string) Opt {
	return func(ts *tokenSource) {
		if aud != "" {
			ts.audience = aud
		}
	}
}

type tokenPayload struct {
	Audience string `json:"aud"`
}

// NewTokenSource: https://fly.io/docs/security/openid-connect/
func NewTokenSource(opts ...Opt) oauth2.TokenSource {
	source := &tokenSource{
		client: http2.NewUnixSocketClient("/.fly/api"),
	}
	for _, opt := range opts {
		opt(source)
	}

	return oauth2.ReuseTokenSource(nil, source)
}

func (ts *tokenSource) Token() (*oauth2.Token, error) {
	tokenURL := &url.URL{
		Scheme: "http",
		Host:   "localhost",
		Path:   "/v1/tokens/oidc",
	}

	payload := &tokenPayload{
		Audience: ts.audience,
	}

	payloadData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	tokenReq, err := http.NewRequest("POST", tokenURL.String(), bytes.NewReader(payloadData))
	if err != nil {
		return nil, err
	}

	tokenReq.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(tokenReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	token := &oauth2.Token{}
	responseBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(responseBytes, token)
	if err != nil {
		return nil, err
	}

	return token, nil
}
