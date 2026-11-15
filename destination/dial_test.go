package destination

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return srv
}

// get returns the status code of the final response.
func get(t *testing.T, client *http.Client, target string) (int, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode, nil
}

func ok() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
}

func Test_Policy_Client_LoopbackDestination(t *testing.T) {
	srv := newServer(t, ok())

	t.Run("refused by default", func(t *testing.T) {
		_, err := get(t, Policy{}.Client(0), srv.URL)

		require.ErrorIs(t, err, ErrDenied)
	})

	t.Run("permitted when allow-listed", func(t *testing.T) {
		policy := Policy{AllowPrefixes: prefixes("127.0.0.0/8")}

		status, err := get(t, policy.Client(0), srv.URL)

		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, status)
	})
}

func Test_Policy_Client_RedirectToDeniedAddress(t *testing.T) {
	denied := newServer(t, ok())
	deniedURL, err := url.Parse(denied.URL)
	require.NoError(t, err)
	// 127.0.0.2 is not in the allow-list below, so the redirect target is
	// refused before the socket connects, whether or not anything listens.
	target := "http://127.0.0.2:" + deniedURL.Port()

	redirector := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusFound)
	}))
	policy := Policy{AllowPrefixes: prefixes("127.0.0.1/32")}

	_, err = get(t, policy.Client(5), redirector.URL)

	require.ErrorIs(t, err, ErrDenied)
}

func Test_Policy_Client_Redirects(t *testing.T) {
	policy := Policy{AllowPrefixes: prefixes("127.0.0.0/8")}
	srv := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop", http.StatusFound)
	}))

	t.Run("zero follows none", func(t *testing.T) {
		status, err := get(t, policy.Client(0), srv.URL+"/loop")

		require.NoError(t, err)
		assert.Equal(t, http.StatusFound, status)
	})

	t.Run("limit stops a loop", func(t *testing.T) {
		_, err := get(t, policy.Client(3), srv.URL+"/loop")

		require.ErrorContains(t, err, "stopped after 3 redirects")
	})
}

func Test_Policy_Client_Scheme(t *testing.T) {
	_, err := get(t, Policy{}.Client(0), "file:///etc/passwd")

	require.ErrorIs(t, err, ErrDenied)
}

func Test_Policy_Dialer_ResolvedHostname(t *testing.T) {
	srv := newServer(t, ok())
	srvURL, err := url.Parse(srv.URL)
	require.NoError(t, err)
	address := net.JoinHostPort("internal.test", srvURL.Port())
	resolver := fakeResolver(t, "127.0.0.1")

	t.Run("hostname resolving to a denied address is refused", func(t *testing.T) {
		dialer := Policy{}.Dialer()
		dialer.Resolver = resolver

		_, err := dialer.DialContext(context.Background(), "tcp4", address)

		require.ErrorIs(t, err, ErrDenied)
	})

	t.Run("hostname resolving to an allow-listed address connects", func(t *testing.T) {
		dialer := Policy{AllowPrefixes: prefixes("127.0.0.0/8")}.Dialer()
		dialer.Resolver = resolver

		conn, err := dialer.DialContext(context.Background(), "tcp4", address)

		require.NoError(t, err)
		require.NoError(t, conn.Close())
	})
}
