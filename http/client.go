package http

import (
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/dioad/generics"

	"github.com/dioad/net/http/auth"
	"github.com/dioad/net/http/auth/basic"
)

type Client struct {
	Config *ClientConfig
}

type ClientConfig struct {
	BaseURL    *url.URL
	Client     *http.Client
	UserAgent  string
	AuthConfig auth.ClientConfig
}

func (c *Client) checkConfig() error {
	if c.Config == nil {
		return fmt.Errorf("no config not specified for client")
	}

	if c.Config.Client == nil {
		// Do we just want to use the default one here instead of failing?
		return fmt.Errorf("no HTTP client specified for client")
	}

	if c.Config.BaseURL == nil {
		return fmt.Errorf("no base url specified for client")
	}

	return nil
}

func (c *Client) Request(req *http.Request) (*http.Response, error) {
	if err := c.checkConfig(); err != nil {
		return nil, err
	}

	libraryUserAgent := "DioadClient/VERSION"

	if c.Config.UserAgent != "" {
		req.Header.Set("User-Agent", fmt.Sprintf("%s %s", c.Config.UserAgent, libraryUserAgent))
	} else {
		req.Header.Set("User-Agent", libraryUserAgent)
	}

	req.Header.Set("Content-Type", "application/json")

	if !generics.IsZeroValue(c.Config.AuthConfig) {
		ac := auth.AuthClient(c.Config.AuthConfig)

		err := ac.AddAuth(req)
		if err != nil {
			return nil, err
		}
	}

	// Add basic / netrc credentials to the request if they exist
	basic.AddCredentials(req)

	return c.Config.Client.Do(req)
}

func (c *Client) ResolveRelativeRequestPath(requestPath string) (*url.URL, error) {
	if err := c.checkConfig(); err != nil {
		return nil, err
	}

	relativePathURL, err := url.Parse(requestPath)
	if err != nil {
		return nil, err
	}

	return c.Config.BaseURL.ResolveReference(relativePathURL), nil
}

func NewDefaultClient() *Client {
	return NewClient(&ClientConfig{
		Client:    &http.Client{},
		UserAgent: "",
	})
}

func NewClient(config *ClientConfig) *Client {
	log.Printf("%v", config)
	return &Client{
		Config: config,
	}
}
