package oidc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/dioad/net/oidc/flyio"
)

type clientConfigTokenSource struct {
	clientConfig ClientConfig
	tokenSource  oauth2.TokenSource

	m sync.Mutex
}

func (c *clientConfigTokenSource) resolveTokenSource() (oauth2.TokenSource, error) {
	if c.clientConfig.Type == "flyio" {
		return flyio.NewTokenSource(flyio.WithAudience(c.clientConfig.Audience)), nil
	}

	oidcClient, err := NewClientFromConfig(&c.clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC client: %w", err)
	}
	if c.clientConfig.TokenFile != "" {
		token, err := ResolveTokenFromFile(c.clientConfig.TokenFile)
		if err == nil {
			if token.AccessToken != "" && token.RefreshToken == "" {
				return oauth2.StaticTokenSource(token), nil
			}
			return oidcClient.TokenSource(token)
		}
	}

	if c.clientConfig.ClientID != "" && c.clientConfig.ClientSecret.MaskedString() != "" {
		token, err := oidcClient.ClientCredentialsToken(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get client credentials token: %w", err)
		}
		return oauth2.StaticTokenSource(token), nil
	}

	return nil, fmt.Errorf("no token source found")
}

func (c *clientConfigTokenSource) Token() (*oauth2.Token, error) {
	c.m.Lock()
	defer c.m.Unlock()
	if c.tokenSource != nil {
		return c.tokenSource.Token()
	}

	var err error
	c.tokenSource, err = c.resolveTokenSource()
	if err != nil {
		return nil, fmt.Errorf("failed to get identity token: %w", err)
	}

	return c.tokenSource.Token()
}

func NewTokenSourceFromConfig(config ClientConfig) oauth2.TokenSource {
	return &clientConfigTokenSource{
		clientConfig: config,
	}
}

type waitingTokenSource struct {
	sourceTokenSource oauth2.TokenSource
	tokenSource       oauth2.TokenSource
	Interval          time.Duration
	MaxTime           time.Duration
	parentCtx         context.Context
}

func waitForToken(ctx context.Context, tokenSource oauth2.TokenSource, interval time.Duration, maxTime time.Duration) (oauth2.TokenSource, error) {
	token, err := tokenSource.Token()
	if err == nil && token.Valid() {
		return tokenSource, nil
	}

	t := time.NewTicker(interval)

	for {
		var err error
		select {
		case <-t.C:
			token, err := tokenSource.Token()
			if err == nil && token.Valid() {
				return tokenSource, nil
			}
		case <-time.After(maxTime):
			if err != nil {
				return nil, fmt.Errorf("timed out waiting for identity token after %v: %w", maxTime, err)
			}
			return nil, fmt.Errorf("timed out waiting for identity token after %v", maxTime)
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled while waiting for identity token")
		}
	}
}

func (w *waitingTokenSource) Token() (*oauth2.Token, error) {
	if w.tokenSource != nil {
		return w.tokenSource.Token()
	}

	tokenSource, err := waitForToken(w.parentCtx, w.sourceTokenSource, w.Interval, w.MaxTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get identity token: %w", err)
	}

	w.tokenSource = tokenSource
	return w.tokenSource.Token()
}

func NewWaitingTokenSource(ctx context.Context, tokenSource oauth2.TokenSource, interval time.Duration, maxTime time.Duration) oauth2.TokenSource {
	return &waitingTokenSource{
		sourceTokenSource: tokenSource,
		parentCtx:         ctx,
		Interval:          interval,
		MaxTime:           maxTime,
	}
}

func NewWaitingTokenSourceFromConfig(ctx context.Context, config ClientConfig, interval time.Duration, maxTime time.Duration) oauth2.TokenSource {
	tokenSource := NewTokenSourceFromConfig(config)

	return NewWaitingTokenSource(ctx, tokenSource, interval, maxTime)
}
