package metrics

import (
	"bytes"
	"io"
	"net"
	"sync"
	"testing"
	"time"

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

func TestConnDuration(t *testing.T) {
	controlConn, testConn := net.Pipe()
	c := NewConn(testConn)

	conn, ok := c.(*Conn)
	require.True(t, ok, "NewConn must return a *Conn")

	wg := sync.WaitGroup{}

	// startTime := time.Now()
	wg.Go(func() {
		controlBytes := make([]byte, 1)
		_, _ = controlConn.Write([]byte("a"))
		_, _ = controlConn.Read(controlBytes)
	})

	var midDuration time.Duration
	var endDuration time.Duration

	wg.Go(func() {

		dest := make([]byte, 1)
		time.Sleep(50 * time.Millisecond)

		_, _ = c.Read(dest)

		midDuration = conn.Duration()

		time.Sleep(50 * time.Millisecond)

		_, _ = c.Write([]byte("b"))

		endDuration = conn.Duration()
	})

	wg.Wait()

	roundedMidDuration := midDuration.Truncate(10 * time.Millisecond)
	if roundedMidDuration != 50*time.Millisecond {
		t.Errorf("middle duration mismatch: %v(rounded=%v) != %v", midDuration, roundedMidDuration, 100*time.Millisecond)
	}

	roundedEndDuration := endDuration.Truncate(10 * time.Millisecond)
	if roundedEndDuration != 100*time.Millisecond {
		t.Errorf("end duration mismatch: %v(rounded=%v) != %v", endDuration, roundedEndDuration, 200*time.Millisecond)
	}

	_ = conn.Close()

	// roundedD1 := d1.Round(10 * time.Millisecond)
	// if roundedD1 != 250*time.Millisecond {

	// }

	// roundedD2 := d2.Round(10 * time.Millisecond)
	// if roundedD2 != 500*time.Millisecond {

	// }
}

func TestConnDuration_ZeroBeforeAnyIO(t *testing.T) {
	_, testConn := net.Pipe()
	c := NewConn(testConn)
	defer func() { _ = c.Close() }()

	conn, ok := c.(*Conn)
	require.True(t, ok, "NewConn must return a *Conn")
	assert.Equal(t, time.Duration(0), conn.Duration(), "Duration before any read/write must be 0, not endTime.Sub(startTime) with a zero endTime")
}

func TestConnBytesWritten(t *testing.T) {
	server, client := net.Pipe()
	c := NewConn(client)

	bytesToWrite := []byte("hello")

	go func() {
		_, _ = c.Write([]byte("hello"))
		_ = c.Close()
	}()
	bytesWritten, _ := io.ReadAll(server)

	if !bytes.Equal(bytesWritten, bytesToWrite) {
		t.Fatalf("failed to pass-through write")
	}

	conn, ok := c.(*Conn)
	require.True(t, ok, "NewConn must return a *Conn")

	if uint64(len(bytesWritten)) != conn.BytesWritten() {
		t.Fatalf("c.BytesWritten() not equal to bytes written")
	}
}

func TestConnBytesRead(t *testing.T) {
	server, client := net.Pipe()
	c := NewConn(client)

	bytesToWrite := []byte("hello")

	go func() {
		_, _ = server.Write([]byte("hello"))
		_ = server.Close()
	}()
	bytesWritten, _ := io.ReadAll(c)

	if !bytes.Equal(bytesWritten, bytesToWrite) {
		t.Fatalf("failed to pass-through read")
	}

	conn, ok := c.(*Conn)
	require.True(t, ok, "NewConn must return a *Conn")

	if uint64(len(bytesToWrite)) != conn.BytesRead() {
		t.Fatalf("c.BytesRead() not equal to bytes read")
	}
}

func TestConn_CloseWrite_DelegatesWhenSupported(t *testing.T) {
	t.Parallel()

	fake := &fakeCloseWriteConn{}
	c := NewConn(fake)

	closeWriter, ok := c.(interface{ CloseWrite() error })
	require.True(t, ok, "Conn must implement CloseWrite() error")

	err := closeWriter.CloseWrite()

	require.NoError(t, err)
	assert.True(t, fake.closeWriteCalled, "expected CloseWrite to delegate to the wrapped conn's own CloseWrite")
	assert.False(t, fake.closeCalled, "delegating to a real half-close must not also fully close the wrapped conn")

	conn, ok := c.(*Conn)
	require.True(t, ok, "NewConn must return a *Conn")
	assert.False(t, conn.conn.Closed(), "CloseWrite must not mark the connection as closed when it only half-closed the wrapped conn")
}

func TestConn_CloseWrite_FallsBackToCloseWhenUnsupported(t *testing.T) {
	t.Parallel()

	// net.Pipe conns implement neither CloseWrite nor CloseRead - a
	// representative stand-in for connection types like *yamux.Stream that
	// have no half-close primitive at all.
	_, client := net.Pipe()
	c := NewConn(client)

	closeWriter, ok := c.(interface{ CloseWrite() error })
	require.True(t, ok, "Conn must implement CloseWrite() error")

	err := closeWriter.CloseWrite()

	require.NoError(t, err)

	conn, ok := c.(*Conn)
	require.True(t, ok, "NewConn must return a *Conn")
	assert.True(t, conn.conn.Closed(), "CloseWrite must fall back to a full Close() when the wrapped conn has no half-close of its own")
}
