package net

import (
	"bytes"
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnCloser(t *testing.T) {
	result := false

	_, client := net.Pipe()
	c := NewConnWithCloser(client, func(c net.Conn) { result = true })

	_ = c.Close()

	if !result {
		t.Fatalf("failed to call closer")
	}
}

func TestConnCloserWithNil(t *testing.T) {
	_, client := net.Pipe()
	c := NewConnWithCloser(client, nil)

	err := c.Close()

	if err != nil {
		t.Fatalf("close failed")
	}
}

func TestConnCloserPassThroughWrite(t *testing.T) {
	server, client := net.Pipe()
	c := NewConnWithCloser(client, func(c net.Conn) {})

	bytesToWrite := []byte("hello")

	go func() {
		_, _ = c.Write([]byte("hello"))
		_ = c.Close()
	}()
	bytesWritten, _ := io.ReadAll(server)

	if !bytes.Equal(bytesWritten, bytesToWrite) {
		t.Fatalf("failed to pass-through write")
	}
}

func TestConnCloserPassThroughRead(t *testing.T) {
	server, client := net.Pipe()
	c := NewConnWithCloser(client, func(c net.Conn) {})

	bytesToWrite := []byte("hello")

	go func() {
		_, _ = server.Write([]byte("hello"))
		_ = server.Close()
	}()
	bytesWritten, _ := io.ReadAll(c)

	if !bytes.Equal(bytesWritten, bytesToWrite) {
		t.Fatalf("failed to pass-through read")
	}
}

func TestConnWithCloser_CloseWrite_DelegatesWhenSupported(t *testing.T) {
	t.Parallel()

	fake := &fakeCloseWriteConn{}
	c := NewConnWithCloser(fake, nil)

	err := c.(interface{ CloseWrite() error }).CloseWrite()

	require.NoError(t, err)
	assert.True(t, fake.closeWriteCalled, "expected CloseWrite to delegate to the wrapped conn's own CloseWrite")
	assert.False(t, fake.closeCalled, "delegating to a real half-close must not also fully close the wrapped conn")
	assert.False(t, c.Closed(), "CloseWrite must not mark the connWithCloser itself as closed when it only half-closed the wrapped conn")
}

func TestConnWithCloser_CloseWrite_FallsBackToCloseWhenUnsupported(t *testing.T) {
	t.Parallel()

	_, client := net.Pipe()
	c := NewConnWithCloser(client, nil)

	err := c.(interface{ CloseWrite() error }).CloseWrite()

	require.NoError(t, err)
	assert.True(t, c.Closed(), "CloseWrite must fall back to a full Close() when the wrapped conn has no half-close of its own")
}
