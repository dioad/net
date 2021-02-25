package metrics

import (
	"bytes"
	"io/ioutil"
	"net"
	"testing"
	"time"
)

func TestConnDuration(t *testing.T) {
	_, client := net.Pipe()
	c := NewConn(client)

	time.Sleep(250 * time.Millisecond)
	d1 := c.(*Conn).Duration()
	time.Sleep(250 * time.Millisecond)
	c.(*Conn).Close()
	d2 := c.(*Conn).Duration()
	time.Sleep (100 * time.Millisecond)

	roundedD1 := d1.Round(10 * time.Millisecond)
	if roundedD1 != 250 * time.Millisecond {
		t.Fatalf( "middle duration mismatch: %v != %v", roundedD1, 250 * time.Millisecond)
	}

	roundedD2 := d2.Round(10 * time.Millisecond)
	if roundedD2 != 500 * time.Millisecond {
		t.Fatalf( "end duration mismatch: %v != %v", roundedD2, 500 * time.Millisecond)
	}
}

func TestConnBytesWritten(t *testing.T) {
	server, client := net.Pipe()
	c := NewConn(client)

	bytesToWrite := []byte("hello")

	go func() {
		c.Write([]byte("hello"))
		c.Close()
	}()
	bytesWritten, _ := ioutil.ReadAll(server)

	if !bytes.Equal(bytesWritten, bytesToWrite) {
		t.Fatalf("failed to pass-through write")
	}

	if len(bytesWritten) !=  c.(*Conn).BytesWritten() {
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
	bytesWritten, _ := ioutil.ReadAll(c)

	if !bytes.Equal(bytesWritten, bytesToWrite) {
		t.Fatalf("failed to pass-through read")
	}

    if len(bytesToWrite) !=  c.(*Conn).BytesRead() {
		t.Fatalf("c.BytesRead() not equal to bytes read")
	}
}
