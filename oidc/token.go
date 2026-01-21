package oidc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/oauth2"

	"github.com/dioad/net/oidc/flyio"
	"github.com/dioad/net/oidc/githubactions"
)

type clientConfigTokenSource struct {
	clientConfig ClientConfig
	tokenSource  oauth2.TokenSource

	m sync.Mutex
}

// resolvePlatformTokenSource resolves token sources for platform-specific providers (Fly.io, GitHub Actions)
func (c *clientConfigTokenSource) resolvePlatformTokenSource() (oauth2.TokenSource, bool) {
	if c.clientConfig.Type == "flyio" {
		return flyio.NewTokenSource(flyio.WithAudience(c.clientConfig.Audience)), true
	}

	if c.clientConfig.Type == "githubactions" {
		return githubactions.NewTokenSource(githubactions.WithAudience(c.clientConfig.Audience)), true
	}

	return nil, false
}

// resolveFileTokenSource resolves token source from a token file, if configured
func (c *clientConfigTokenSource) resolveFileTokenSource(oidcClient *Client) (oauth2.TokenSource, bool, error) {
	if c.clientConfig.TokenFile == "" {
		return nil, false, nil
	}

	token, err := ResolveTokenFromFile(c.clientConfig.TokenFile)
	if err != nil {
		return nil, false, nil
	}

	if token.AccessToken != "" && token.RefreshToken == "" {
		return oauth2.StaticTokenSource(token), true, nil
	}
	
	tokenSource, err := oidcClient.TokenSource(token)
	if err != nil {
		return nil, false, err
	}
	return tokenSource, true, nil
}

// resolveClientCredentialsTokenSource resolves token source using client credentials grant
func (c *clientConfigTokenSource) resolveClientCredentialsTokenSource(oidcClient *Client) (oauth2.TokenSource, bool, error) {
	if c.clientConfig.ClientID == "" || c.clientConfig.ClientSecret.UnmaskedString() == "" {
		return nil, false, nil
	}

	token, err := oidcClient.RefreshingClientCredentialsToken(context.Background())
	if err != nil {
		return nil, false, fmt.Errorf("failed to get client credentials token: %w", err)
	}

	return token, true, nil
}

func (c *clientConfigTokenSource) resolveTokenSource() (oauth2.TokenSource, error) {
	// Try platform-specific token sources first
	if tokenSource, ok := c.resolvePlatformTokenSource(); ok {
		return tokenSource, nil
	}

	// For other token sources, we need an OIDC client
	oidcClient, err := NewClientFromConfig(&c.clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC client: %w", err)
	}

	// Try file-based token source
	if tokenSource, ok, err := c.resolveFileTokenSource(oidcClient); err != nil {
		return nil, err
	} else if ok {
		return tokenSource, nil
	}

	// Try client credentials token source
	if tokenSource, ok, err := c.resolveClientCredentialsTokenSource(oidcClient); err != nil {
		return nil, err
	} else if ok {
		return tokenSource, nil
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
	attempt := 0

	logger := zerolog.Ctx(ctx)
	token, err := tokenSource.Token()
	if err == nil && token.Valid() {
		logger.Trace().Msgf("token valid")
		return tokenSource, nil
	}

	t := time.NewTicker(interval)

	for {
		attempt++
		attemptLogger := logger.With().Dur("interval", interval).Int("attempt", attempt).Logger()
		var err error
		select {
		case <-t.C:
			token, err := tokenSource.Token()
			if err == nil && token.Valid() {
				attemptLogger.Trace().Msgf("token valid")
				return tokenSource, nil
			}
			attemptLogger.Trace().Err(err).Msgf("token invalid")
		case <-time.After(maxTime):
			if err != nil {
				attemptLogger.Trace().Err(err).Dur("timeout", maxTime).Msgf("token timeout")
				return nil, fmt.Errorf("timed out waiting for identity token after %v: %w", maxTime, err)
			}
			attemptLogger.Trace().Dur("timeout", maxTime).Msgf("token timeout")
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
