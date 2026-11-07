package tls

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const invalidTLSALPNAuthorization = `{
  "identifier": {"type": "dns", "value": "example.com"},
  "status": "invalid",
  "challenges": [
    {
      "type": "tls-alpn-01",
      "url": "https://ca.example/chall/1",
      "status": "invalid",
      "error": {
        "type": "urn:ietf:params:acme:error:unauthorized",
        "detail": "Incorrect validation certificate for tls-alpn-01 challenge"
      },
      "validationRecord": [
        {"hostname": "example.com", "port": "443", "addressUsed": "192.0.2.1", "addressesResolved": ["192.0.2.1", "2001:db8::1"]}
      ]
    },
    {"type": "http-01", "url": "https://ca.example/chall/2", "status": "pending"}
  ]
}`

func TestACMEProblemLogger(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		contentType string
		status      int
		body        string
		wantEvents  []map[string]any
	}{
		{
			name:        "an invalid authorization logs the failed challenge with the CA's reason",
			contentType: "application/json",
			status:      http.StatusOK,
			body:        invalidTLSALPNAuthorization,
			wantEvents: []map[string]any{{
				"level":                              "warn",
				"message":                            "acme challenge failed",
				"acme_identifier":                    "example.com",
				"acme_challenge_type":                "tls-alpn-01",
				"acme_challenge_url":                 "https://ca.example/chall/1",
				"acme_problem_type":                  "urn:ietf:params:acme:error:unauthorized",
				"acme_problem_detail":                "Incorrect validation certificate for tls-alpn-01 challenge",
				"acme_validation_hostnames":          []any{"example.com"},
				"acme_validation_addresses_used":     []any{"192.0.2.1"},
				"acme_validation_addresses_resolved": []any{"192.0.2.1", "2001:db8::1"},
			}},
		},
		{
			name:        "an invalid authorization with no challenge error is still reported",
			contentType: "application/json",
			status:      http.StatusOK,
			body:        `{"identifier": {"type": "dns", "value": "example.com"}, "status": "invalid", "challenges": [{"type": "tls-alpn-01", "status": "pending"}]}`,
			wantEvents: []map[string]any{{
				"level":           "warn",
				"message":         "acme authorization invalid",
				"acme_identifier": "example.com",
			}},
		},
		{
			name:        "an invalid challenge logs the CA's reason",
			contentType: "application/json",
			status:      http.StatusOK,
			body:        `{"type": "tls-alpn-01", "url": "https://ca.example/chall/1", "status": "invalid", "error": {"type": "urn:ietf:params:acme:error:connection", "detail": "Timeout during connect"}}`,
			wantEvents: []map[string]any{{
				"level":               "warn",
				"message":             "acme challenge failed",
				"acme_challenge_type": "tls-alpn-01",
				"acme_problem_type":   "urn:ietf:params:acme:error:connection",
				"acme_problem_detail": "Timeout during connect",
			}},
		},
		{
			name:        "an invalid order logs its identifiers and the CA's reason",
			contentType: "application/json",
			status:      http.StatusOK,
			body:        `{"status": "invalid", "identifiers": [{"type": "dns", "value": "example.com"}], "error": {"type": "urn:ietf:params:acme:error:caa", "detail": "CAA record forbids issuance"}}`,
			wantEvents: []map[string]any{{
				"level":               "warn",
				"message":             "acme order invalid",
				"acme_identifiers":    []any{"example.com"},
				"acme_problem_type":   "urn:ietf:params:acme:error:caa",
				"acme_problem_detail": "CAA record forbids issuance",
			}},
		},
		{
			name:        "a rejected request logs the problem and its subproblems",
			contentType: "application/problem+json",
			status:      http.StatusForbidden,
			body:        `{"type": "urn:ietf:params:acme:error:rejectedIdentifier", "detail": "Error creating new order", "subproblems": [{"type": "urn:ietf:params:acme:error:rejectedIdentifier", "detail": "policy forbids issuing for name", "identifier": {"type": "dns", "value": "example.com"}}]}`,
			wantEvents: []map[string]any{{
				"level":               "warn",
				"message":             "acme request rejected",
				"http_status":         float64(http.StatusForbidden),
				"acme_problem_type":   "urn:ietf:params:acme:error:rejectedIdentifier",
				"acme_problem_detail": "Error creating new order",
				"acme_subproblems":    []any{"example.com: urn:ietf:params:acme:error:rejectedIdentifier: policy forbids issuing for name"},
			}},
		},
		{
			name:        "a stale nonce, which the client retries, is only debug",
			contentType: "application/problem+json",
			status:      http.StatusBadRequest,
			body:        `{"type": "urn:ietf:params:acme:error:badNonce", "detail": "JWS has an invalid anti-replay nonce"}`,
			wantEvents: []map[string]any{{
				"level":             "debug",
				"message":           "acme request rejected",
				"acme_problem_type": "urn:ietf:params:acme:error:badNonce",
			}},
		},
		{
			name:        "a valid authorization logs nothing",
			contentType: "application/json",
			status:      http.StatusOK,
			body:        `{"identifier": {"type": "dns", "value": "example.com"}, "status": "valid", "challenges": []}`,
		},
		{
			name:        "a certificate chain is not inspected",
			contentType: "application/pem-certificate-chain",
			status:      http.StatusOK,
			body:        "-----BEGIN CERTIFICATE-----\n",
		},
		{
			name:        "a body too large to inspect is passed through",
			contentType: "application/json",
			status:      http.StatusOK,
			body:        `{"status": "invalid", "padding": "` + strings.Repeat("x", acmeMaxInspectedBody) + `"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			}))
			t.Cleanup(server.Close)

			var logs bytes.Buffer
			ctx := zerolog.New(&logs).Level(zerolog.DebugLevel).WithContext(t.Context())
			client := &http.Client{Transport: newACMEProblemLogger(ctx, http.DefaultTransport)}

			req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL, nil)
			require.NoError(t, err)
			resp, err := client.Do(req)
			require.NoError(t, err)
			t.Cleanup(func() { _ = resp.Body.Close() })

			got, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, tt.body, string(got), "the caller must read the response exactly as the CA sent it")

			events := decodeLogEvents(t, &logs)
			require.Len(t, events, len(tt.wantEvents))
			for i, want := range tt.wantEvents {
				for field, value := range want {
					assert.Equal(t, value, events[i][field], "log field %q", field)
				}
			}
		})
	}
}

func decodeLogEvents(t *testing.T, logs *bytes.Buffer) []map[string]any {
	t.Helper()

	var events []map[string]any
	scanner := bufio.NewScanner(logs)
	for scanner.Scan() {
		var event map[string]any
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &event))
		events = append(events, event)
	}
	require.NoError(t, scanner.Err())

	return events
}
