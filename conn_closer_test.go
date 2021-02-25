package net

import (
	"bytes"
	"io/ioutil"
	"net"
	"testing"
)

func TestConnCloser(t *testing.T) {
	result := false

	_, client := net.Pipe()
	c := NewConnWithCloser(client, func(c net.Conn) { result = true })

	c.Close()

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
		c.Write([]byte("hello"))
		c.Close()
	}()
	bytesWritten, _ := ioutil.ReadAll(server)

	if !bytes.Equal(bytesWritten, bytesToWrite) {
		t.Fatalf("failed to pass-through write")
	}
}

func TestConnCloserPassThroughRead(t *testing.T) {
	server, client := net.Pipe()
	c := NewConnWithCloser(client, func(c net.Conn) {})

	bytesToWrite := []byte("hello")

	go func() {
		server.Write([]byte("hello"))
		server.Close()
	}()
	bytesWritten, _ := ioutil.ReadAll(c)

	if !bytes.Equal(bytesWritten, bytesToWrite) {
		t.Fatalf("failed to pass-through read")
	}
}
