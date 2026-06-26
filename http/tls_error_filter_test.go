package http

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsBenignTLSHandshakeReason(t *testing.T) {
	t.Parallel()
	cases := []struct {
		reason string
		want   bool
	}{
		{"i/o deadline reached", true},
		{"i/o timeout", true},
		{"connection reset by peer", true},
		{"EOF", true},
		{"tls: client requested unsupported application protocols ([\"h2c\" \"spdy/3\"])", true},
		{"tls: client offered only unsupported versions: [302 301]", true},
		{"tls: no cipher suite supported by both client and server", true},
		{"tls: bad record MAC", false},
		{"remote error: tls: certificate required", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.reason, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, isBenignTLSHandshakeReason(tc.reason))
		})
	}
}

func TestTLSHandshakeErrorFilter_benign(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	f := &tlsHandshakeErrorFilter{logger: logger}

	msg := "server.go:3648: http: TLS handshake error from 136.124.32.124:39391: i/o deadline reached\n"
	n, err := f.Write([]byte(msg))
	require.NoError(t, err)
	assert.Equal(t, len(msg), n)

	out := buf.String()
	assert.Contains(t, out, `"level":"warn"`)
	assert.Contains(t, out, `"message":"tls_probe"`)
	assert.Contains(t, out, `"remote_addr":"136.124.32.124:39391"`)
	assert.Contains(t, out, `"reason":"i/o deadline reached"`)
	assert.NotContains(t, out, `"level":"error"`)
}

func TestTLSHandshakeErrorFilter_nonBenign(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	f := &tlsHandshakeErrorFilter{logger: logger}

	msg := "server.go:3648: http: TLS handshake error from 10.0.0.1:1234: tls: bad record MAC\n"
	n, err := f.Write([]byte(msg))
	require.NoError(t, err)
	assert.Equal(t, len(msg), n)

	out := buf.String()
	assert.Contains(t, out, `"level":"error"`)
	assert.NotContains(t, out, `"level":"warn"`)
	assert.NotContains(t, out, `"message":"tls_probe"`)
}

func TestTLSHandshakeErrorFilter_nonTLSMessage(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	f := &tlsHandshakeErrorFilter{logger: logger}

	msg := "server.go:999: http: superfluous response.WriteHeader call\n"
	n, err := f.Write([]byte(msg))
	require.NoError(t, err)
	assert.Equal(t, len(msg), n)

	out := buf.String()
	assert.Contains(t, out, `"level":"error"`)
	assert.NotContains(t, out, `"level":"warn"`)
}

func TestTLSHandshakeErrorFilter_unsupportedProtocols(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	f := &tlsHandshakeErrorFilter{logger: logger}

	msg := "server.go:3648: http: TLS handshake error from 185.213.155.192:44806: tls: client requested unsupported application protocols ([\"http/0.9\" \"spdy/1\" \"h2c\" \"hq\"])\n"
	n, err := f.Write([]byte(msg))
	require.NoError(t, err)
	assert.Equal(t, len(msg), n)

	out := buf.String()
	assert.Contains(t, out, `"level":"warn"`)
	assert.Contains(t, out, `"message":"tls_probe"`)
	assert.Contains(t, out, `"remote_addr":"185.213.155.192:44806"`)
}
