package dns

import (
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
	req, err := doh.NewRequest(http.MethodGet, c.URL.Host, msg)
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
