package dns

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/coredns/coredns/plugin/pkg/doh"
	"github.com/miekg/dns"
)

// DOHClient describes a DNS over HTTPS client.
type DOHClient struct {
	Client *http.Client
	URL    *url.URL
}

// Exchange performs a DNS query using DNS over HTTPS.
func (c *DOHClient) Exchange(msg *dns.Msg) (*dns.Msg, error) {
	// pass Hostname() rather than String() to NewRequest as
	// there's something in there that is acting weird.
	// when passing URL.String() it works on darwin/arm64 but fails on github.com actions
	addr := c.URL.Hostname()
	if c.URL.Port() != "" {
		addr = fmt.Sprintf("%s:%s", addr, c.URL.Port())
	}
	req, err := doh.NewRequest(http.MethodGet, addr, msg)
	if err != nil {
		return nil, err
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}

	msgAnswer, err := doh.ResponseToMsg(resp)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}

	return msgAnswer, resp.Body.Close()
}

// NewDOHClient creates a new DNS over HTTPS client with the provided URL.
func NewDOHClient(url *url.URL) *DOHClient {
	return &DOHClient{
		Client: &http.Client{
			Transport: http.DefaultTransport,
		},
		URL: url,
	}
}
