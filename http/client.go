package http

import (
	"fmt"
	"net/http"
	"net/url"
	"runtime/debug"
)

var libraryUserAgent = func() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return "DioadClient/" + info.Main.Version
	}
	return "DioadClient/dev"
}()

// Client describes an HTTP client for making requests to a base URL.
type Client struct {
	Config *ClientConfig
}

// ClientConfig describes the configuration for an HTTP client.
type ClientConfig struct {
	BaseURL         *url.URL
	Client          *http.Client
	UserAgent       string
	RequestModifier func(*http.Request) error
}

// Request performs an HTTP request with client configuration and authentication.
func (c *Client) Request(req *http.Request) (*http.Response, error) {
	if c.Config.UserAgent != "" {
		req.Header.Set("User-Agent", fmt.Sprintf("%s %s", c.Config.UserAgent, libraryUserAgent))
	} else {
		req.Header.Set("User-Agent", libraryUserAgent)
	}

	if req.Body != nil && req.ContentLength != 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.Config.RequestModifier != nil {
		if err := c.Config.RequestModifier(req); err != nil {
			return nil, err
		}
	}

	return c.Config.Client.Do(req)
}

// ResolveRelativeRequestPath resolves a relative request path against the client's base URL.
// It returns an error if the client was not configured with a BaseURL.
func (c *Client) ResolveRelativeRequestPath(requestPath string) (*url.URL, error) {
	if c.Config.BaseURL == nil {
		return nil, fmt.Errorf("no base url configured for client")
	}
	relativePathURL, err := url.Parse(requestPath)
	if err != nil {
		return nil, err
	}

	return c.Config.BaseURL.ResolveReference(relativePathURL), nil
}

// NewDefaultClient creates a new HTTP client with default configuration.
func NewDefaultClient() *Client {
	c, err := NewClient(&ClientConfig{
		Client:    &http.Client{},
		UserAgent: "",
	})
	if err != nil {
		panic(fmt.Sprintf("NewDefaultClient: invalid default config: %v", err))
	}
	return c
}

// NewClient creates a new HTTP client with the provided configuration.
// It returns an error if the configuration is missing required fields.
// BaseURL is optional; it is only required when calling ResolveRelativeRequestPath.
func NewClient(config *ClientConfig) (*Client, error) {
	if config == nil {
		return nil, fmt.Errorf("no config specified for client")
	}
	if config.Client == nil {
		return nil, fmt.Errorf("no HTTP client specified for client")
	}
	return &Client{Config: config}, nil
}
