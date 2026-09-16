package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T) *Client {
	t.Helper()
	c, err := NewClient(&ClientConfig{Client: &http.Client{}})
	require.NoError(t, err)
	return c
}

func TestClient_Request_DefaultsContentTypeToJSONWhenUnset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("echo-content-type", r.Header.Get("Content-Type"))
	}))
	defer server.Close()

	c := newTestClient(t)
	req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(`{}`))
	require.NoError(t, err)
	req.ContentLength = 2

	resp, err := c.Request(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, "application/json", resp.Header.Get("echo-content-type"))
}

func TestClient_Request_DoesNotOverrideCallerSetContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("echo-content-type", r.Header.Get("Content-Type"))
	}))
	defer server.Close()

	c := newTestClient(t)
	req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("field=value"))
	require.NoError(t, err)
	req.ContentLength = 11
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.Request(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, "application/x-www-form-urlencoded", resp.Header.Get("echo-content-type"), "a caller-set Content-Type must not be overwritten by the client's default")
}
