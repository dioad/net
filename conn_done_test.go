package net

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCloseWriteConn is a net.Conn whose CloseWrite is a true half-close: it
// records that it was called instead of closing the connection, so tests can
// tell delegation apart from a full Close() fallback.
type fakeCloseWriteConn struct {
	net.Conn
	closeWriteCalled bool
	closeCalled      bool
}

func (f *fakeCloseWriteConn) CloseWrite() error {
	f.closeWriteCalled = true
	return nil
}

func (f *fakeCloseWriteConn) Close() error {
	f.closeCalled = true
	return nil
}

func TestDoneConn_CloseWrite_DelegatesWhenSupported(t *testing.T) {
	t.Parallel()

	fake := &fakeCloseWriteConn{}
	d := NewDoneConn(fake)

	err := d.(interface{ CloseWrite() error }).CloseWrite()

	require.NoError(t, err)
	assert.True(t, fake.closeWriteCalled, "expected CloseWrite to delegate to the wrapped conn's own CloseWrite")
	assert.False(t, fake.closeCalled, "delegating to a real half-close must not also fully close the wrapped conn")
	assert.False(t, d.Closed(), "CloseWrite must not mark the DoneConn itself as closed when it only half-closed the wrapped conn")
}

func TestDoneConn_CloseWrite_FallsBackToCloseWhenUnsupported(t *testing.T) {
	t.Parallel()

	// net.Pipe conns implement neither CloseWrite nor CloseRead - a
	// representative stand-in for connection types like *yamux.Stream that
	// have no half-close primitive at all.
	_, client := net.Pipe()
	d := NewDoneConn(client)

	err := d.(interface{ CloseWrite() error }).CloseWrite()

	require.NoError(t, err)
	assert.True(t, d.Closed(), "CloseWrite must fall back to a full Close() when the wrapped conn has no half-close of its own")
}
