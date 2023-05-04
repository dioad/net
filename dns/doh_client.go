package dns

import (
	"net/http"
	"net/url"

	"github.com/coredns/coredns/plugin/pkg/doh"
	"github.com/miekg/dns"
)

type DOHClient struct {
	Client *http.Client
	URL    *url.URL
}

func (c *DOHClient) Exchange(msg *dns.Msg) (*dns.Msg, error) {
	req, err := doh.NewRequest(http.MethodGet, c.URL.String(), msg)
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

func NewDOHClient(url *url.URL) *DOHClient {
	return &DOHClient{
		Client: &http.Client{},
		URL:    url,
	}
}
