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

		midDuration = c.(*Conn).Duration()

		time.Sleep(50 * time.Millisecond)

		_, _ = c.Write([]byte("b"))

		endDuration = c.(*Conn).Duration()
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

	_ = c.(*Conn).Close()

	// roundedD1 := d1.Round(10 * time.Millisecond)
	// if roundedD1 != 250*time.Millisecond {

	// }

	// roundedD2 := d2.Round(10 * time.Millisecond)
	// if roundedD2 != 500*time.Millisecond {

	// }
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

	if uint64(len(bytesWritten)) != c.(*Conn).BytesWritten() {
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

	if uint64(len(bytesToWrite)) != c.(*Conn).BytesRead() {
		t.Fatalf("c.BytesRead() not equal to bytes read")
	}
}

func TestConn_CloseWrite_DelegatesWhenSupported(t *testing.T) {
	t.Parallel()

	fake := &fakeCloseWriteConn{}
	c := NewConn(fake)

	err := c.(interface{ CloseWrite() error }).CloseWrite()

	require.NoError(t, err)
	assert.True(t, fake.closeWriteCalled, "expected CloseWrite to delegate to the wrapped conn's own CloseWrite")
	assert.False(t, fake.closeCalled, "delegating to a real half-close must not also fully close the wrapped conn")
	assert.False(t, c.(*Conn).conn.Closed(), "CloseWrite must not mark the connection as closed when it only half-closed the wrapped conn")
}

func TestConn_CloseWrite_FallsBackToCloseWhenUnsupported(t *testing.T) {
	t.Parallel()

	// net.Pipe conns implement neither CloseWrite nor CloseRead - a
	// representative stand-in for connection types like *yamux.Stream that
	// have no half-close primitive at all.
	_, client := net.Pipe()
	c := NewConn(client)

	err := c.(interface{ CloseWrite() error }).CloseWrite()

	require.NoError(t, err)
	assert.True(t, c.(*Conn).conn.Closed(), "CloseWrite must fall back to a full Close() when the wrapped conn has no half-close of its own")
}
