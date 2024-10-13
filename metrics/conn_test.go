package metrics

import (
	"bytes"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestConnDuration(t *testing.T) {
	controlConn, testConn := net.Pipe()
	c := NewConn(testConn)

	wg := sync.WaitGroup{}

	// startTime := time.Now()
	wg.Add(1)
	go func() {
		defer wg.Done()
		controlBytes := make([]byte, 1)
		controlConn.Write([]byte("a"))
		controlConn.Read(controlBytes)
	}()

	var midDuration time.Duration
	var endDuration time.Duration

	wg.Add(1)
	go func() {
		defer wg.Done()

		dest := make([]byte, 1)
		time.Sleep(50 * time.Millisecond)

		c.Read(dest)

		midDuration = c.(*Conn).Duration()

		time.Sleep(50 * time.Millisecond)

		c.Write([]byte("b"))

		endDuration = c.(*Conn).Duration()
	}()

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
