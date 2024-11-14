package oidc

import (
	"context"
	"net/http"

	stdtls "crypto/tls"

	"golang.org/x/oauth2"

	"github.com/dioad/net/tls"
)

func NewHTTPClientFromConfigWithContext(ctx context.Context, config *ClientConfig) (*http.Client, error) {
	tokenSource := NewTokenSourceFromConfig(*config)

	return oauth2.NewClient(ctx, tokenSource), nil
}

// TODO: Factor these functions better
func ContextWithTLSConfig(tlsConfig *stdtls.Config) context.Context {
	ctx := context.Background()

	if tlsConfig != nil {
		httpClient := http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		}
		ctx = context.WithValue(ctx, oauth2.HTTPClient, &httpClient)
	}
	return ctx
}

// TODO: Factor these functions better
func ContextFromTLSConfig(cfg tls.ClientConfig) (context.Context, error) {
	tlsConfig, err := tls.NewClientTLSConfig(cfg)
	if err != nil {
		return nil, err
	}

	return ContextWithTLSConfig(tlsConfig), nil
}

func NewHTTPClientFromConfig(config *ClientConfig) (*http.Client, error) {
	ctx, err := ContextFromTLSConfig(config.TLSConfig)
	if err != nil {
		return nil, err
	}

	return NewHTTPClientFromConfigWithContext(ctx, config)
}
