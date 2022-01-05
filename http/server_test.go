package http

import (
	"context"
	"testing"

	"golang.org/x/net/nettest"
)

func TestNewDefaultServer(t *testing.T) {
	c := Config{}

	s := newDefaultServer(c)

	ln, _ := nettest.NewLocalListener("tcp4")

	go func() {
		s.Serve(ln)
	}()

	err := s.Shutdown(context.Background())
	if err != nil {
		t.Error(err)
	}
}

func TestMultipleNewDefaultServer(t *testing.T) {
	c := Config{}

	s1 := newDefaultServer(c)
	s2 := newDefaultServer(c)

	ln1, _ := nettest.NewLocalListener("tcp4")
	ln2, _ := nettest.NewLocalListener("tcp4")

	go func() {
		s1.Serve(ln1)
		s2.Serve(ln2)
	}()

	err := s1.Shutdown(context.Background())
	if err != nil {
		t.Error(err)
	}

	err = s2.Shutdown(context.Background())
	if err != nil {
		t.Error(err)
	}
}
