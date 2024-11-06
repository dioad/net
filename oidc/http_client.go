package oidc

import (
	"context"
	"log/slog"
	"net/http"

	"golang.org/x/oauth2"

	"github.com/dioad/net/tls"
)

func Oauth2ClientWithBaseTransport(client *http.Client, baseTransport http.RoundTripper) (*http.Client, error) {
	t := client.Transport.(*oauth2.Transport)
	t.Base = baseTransport
	return client, nil
}

func Oauth2ClientWithTLS(client *http.Client, tlsConfig tls.ClientConfig) (*http.Client, error) {
	tlsClientConfig, err := tls.NewClientTLSConfig(tlsConfig)
	if err != nil {
		return nil, err
	}

	return Oauth2ClientWithBaseTransport(client, &http.Transport{TLSClientConfig: tlsClientConfig})
}

func NewHTTPClientFromConfig(config *ClientConfig) (*http.Client, error) {
	tokenSource := NewTokenSourceFromConfig(*config)

	token, _ := tokenSource.Token()
	// DEBUG
	slog.SetLogLoggerLevel(slog.LevelDebug)

	slog.Debug("NewHTTPClientFromConfig", "token", token.AccessToken)

	ctx := context.Background()

	tlsConfig, err := tls.NewClientTLSConfig(config.TLSConfig)
	if err != nil {
		return nil, err
	}

	if tlsConfig != nil {
		slog.Debug("NewHTTPClientFromConfig", "tlsConfig", tlsConfig)
		httpClient := http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		}
		ctx = context.WithValue(ctx, oauth2.HTTPClient, &httpClient)
	}

	return oauth2.NewClient(ctx, tokenSource), nil
}
