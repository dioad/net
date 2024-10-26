package oidc

import (
	"fmt"
	"time"

	"golang.org/x/oauth2"
)

type clientConfigTokenSource struct {
	clientConfig ClientConfig
	tokenSource  oauth2.TokenSource
}

func (c *clientConfigTokenSource) resolveTokenSource() (oauth2.TokenSource, error) {
	var tokenSource oauth2.TokenSource
	token, err := ResolveToken(c.clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load OIDC token: %w", err)
	} else {
		if token.RefreshToken == "" {
			tokenSource = oauth2.StaticTokenSource(token)
		}
		oidcClient, err := NewClientFromConfig(&c.clientConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create OIDC client: %w", err)
		}
		tokenSource, err = oidcClient.TokenSource(token)
		if err != nil {
			return nil, fmt.Errorf("failed to create OIDC token source: %w", err)
		}
	}
	return tokenSource, nil
}

func (c *clientConfigTokenSource) Token() (*oauth2.Token, error) {
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
}

func waitForToken(tokenSource oauth2.TokenSource, interval time.Duration, maxTime time.Duration) (oauth2.TokenSource, error) {
	t := time.NewTicker(interval)
	defer t.Stop()

	expired := make(chan bool)
	go func() {
		time.Sleep(maxTime)
		close(expired)
	}()

	for {
		var err error
		select {
		case <-t.C:
			token, err := tokenSource.Token()
			if err == nil && token.Valid() {
				return tokenSource, nil
			}
		case <-expired:
			if err != nil {
				return nil, fmt.Errorf("timed out waiting for identity token after %v: %w", maxTime, err)
			}
			return nil, fmt.Errorf("timed out waiting for identity token after %v", maxTime)
		}
	}
}

func (w *waitingTokenSource) Token() (*oauth2.Token, error) {
	if w.tokenSource != nil {
		return w.tokenSource.Token()
	}

	tokenSource, err := waitForToken(w.sourceTokenSource, w.Interval, w.MaxTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get identity token: %w", err)
	}

	w.tokenSource = tokenSource
	return w.tokenSource.Token()
}

func NewWaitingTokenSource(tokenSource oauth2.TokenSource, interval time.Duration, maxTime time.Duration) oauth2.TokenSource {
	return &waitingTokenSource{
		sourceTokenSource: tokenSource,
		Interval:          interval,
		MaxTime:           maxTime,
	}
}

func NewWaitingTokenSourceFromConfig(config ClientConfig, interval time.Duration, maxTime time.Duration) oauth2.TokenSource {
	tokenSource := NewTokenSourceFromConfig(config)

	return NewWaitingTokenSource(tokenSource, interval, maxTime)
}
