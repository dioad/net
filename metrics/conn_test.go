package metrics

import (
	"bytes"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConnDuration(t *testing.T) {
	controlConn, testConn := net.Pipe()
	c := NewConn(testConn)

	wg := sync.WaitGroup{}

	// startTime := time.Now()
	wg.Go(func() {
		controlBytes := make([]byte, 1)
		controlConn.Write([]byte("a"))
		controlConn.Read(controlBytes)
	})

	var midDuration time.Duration
	var endDuration time.Duration

	wg.Go(func() {

		dest := make([]byte, 1)
		time.Sleep(50 * time.Millisecond)

		c.Read(dest)

		midDuration = c.(*Conn).Duration()

		time.Sleep(50 * time.Millisecond)

		c.Write([]byte("b"))

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

	c.(*Conn).Close()

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
		c.Write([]byte("hello"))
		c.Close()
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
		server.Write([]byte("hello"))
		server.Close()
	}()
	bytesWritten, _ := io.ReadAll(c)

	if !bytes.Equal(bytesWritten, bytesToWrite) {
		t.Fatalf("failed to pass-through read")
	}

	if uint64(len(bytesToWrite)) != c.(*Conn).BytesRead() {
		t.Fatalf("c.BytesRead() not equal to bytes read")
	}
}

func TestIsTLSCloseNotifyError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "tls close_notify with connection closed anyway",
			err:  errors.New("tls: failed to send closeNotify alert (but connection was closed anyway): write tcp 10.0.0.1:443->1.2.3.4:5678: write: broken pipe"),
			want: true,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "net.ErrClosed",
			err:  net.ErrClosed,
			want: false,
		},
		{
			name: "generic broken pipe",
			err:  errors.New("write: broken pipe"),
			want: false,
		},
		{
			name: "other tls error without closed-anyway phrase",
			err:  errors.New("tls: bad record MAC"),
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, isTLSCloseNotifyError(tc.err))
		})
	}
}
