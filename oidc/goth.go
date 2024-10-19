package oidc

import (
	"net/url"

	"github.com/markbates/goth/providers/openidConnect"
)

func NewGothProviderFromConfig(c *ClientConfig, callbackURL *url.URL, scopes ...string) (*openidConnect.Provider, error) {
	client, err := NewClientFromConfig(c)
	if err != nil {
		return nil, err
	}
	return NewGothProvider(
		client,
		callbackURL,
		scopes...)
}

func NewGothProvider(c *Client, callbackURL *url.URL, scopes ...string) (*openidConnect.Provider, error) {
	discoveryEndpoint, err := c.endpoint.DiscoveryEndpoint()
	if err != nil {
		return nil, err
	}

	return openidConnect.New(
		c.clientID,
		c.clientSecret,
		callbackURL.String(),
		discoveryEndpoint.String(),
		scopes...)

}
