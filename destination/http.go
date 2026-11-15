package destination

import (
	"errors"
	"fmt"
	"net/http"
)

// Transport returns an http.RoundTripper that enforces the policy on every
// connection it makes, and refuses any scheme other than http and https. It
// does not use a proxy from the environment: a proxy would be the address
// dialled, and the destination behind it would escape the policy.
//
// A kept-alive connection is not re-checked per request. A live connection
// cannot rebind to another peer, so the check made when it was opened still
// holds; a new connection resolves and checks afresh.
func (p Policy) Transport() http.RoundTripper {
	transport := http.DefaultTransport.(*http.Transport).Clone() //nolint:forcetypeassert // the default is documented to be a *http.Transport
	transport.Proxy = nil
	transport.DialContext = p.Dialer().DialContext

	return schemeCheck{next: transport}
}

// Client returns an http.Client using Transport that follows at most
// maxRedirects redirects. With zero it follows none and returns the redirect
// response itself. A redirect to a denied address is refused at the dial.
func (p Policy) Client(maxRedirects int) *http.Client {
	return &http.Client{
		Transport: p.Transport(),
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) > maxRedirects {
				if maxRedirects == 0 {
					return http.ErrUseLastResponse
				}

				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}

			return nil
		},
	}
}

type schemeCheck struct {
	next http.RoundTripper
}

func (s schemeCheck) RoundTrip(req *http.Request) (*http.Response, error) {
	switch req.URL.Scheme {
	case "http", "https":
		return s.next.RoundTrip(req)
	default:
		return nil, errors.Join(ErrDenied, fmt.Errorf("scheme %q is not http or https", req.URL.Scheme))
	}
}
