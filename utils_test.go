package net

import (
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
)

//style borrowed from https://dave.cheney.net/2019/05/07/prefer-table-driven-tests
func TestTcpPortFromURL(t *testing.T) {
	tests := map[string]struct {
		input url.URL
		want  string
	}{
		"explicit port":  {input: url.URL{Scheme: "http", Host: "example.com:50"}, want: "50"},
		"known scheme":   {input: url.URL{Scheme: "https", Host: "example.com"}, want: "443"},
		"unknown scheme": {input: url.URL{Scheme: "fandango", Host: "example.com"}, want: ""},
		"no scheme":      {input: url.URL{Host: "example.com"}, want: "0"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, _ := TCPPortFromURL(&tc.input)
			diff := cmp.Diff(tc.want, got)
			if diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}

func TestTcpAddrFromURL(t *testing.T) {
	tests := map[string]struct {
		input url.URL
		want  string
	}{
		"explicit port":  {input: url.URL{Scheme: "http", Host: "example.com:50"}, want: "example.com:50"},
		"known scheme":   {input: url.URL{Scheme: "https", Host: "example.com"}, want: "example.com:443"},
		"unknown scheme": {input: url.URL{Scheme: "fandango", Host: "example.com"}, want: ""},
		"no scheme":      {input: url.URL{Host: "example.com"}, want: "example.com:0"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, _ := TCPAddrFromURL(&tc.input)
			diff := cmp.Diff(tc.want, got)
			if diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}


//
//func TestTcpPortFromURL(t *testing.T) {
//	type test struct {
//		input url.URL
//		want  string
//	}
//
//	tests := []test{
//		{input: url.URL{Scheme: "http", Host: "example.com:50"}, want: "50"},
//		{input: url.URL{Scheme: "http", Host: "example.com"}, want: "80"},
//		{input: url.URL{Scheme: "fandango", Host: "example.com:50"}, want: "50"},
//	}
//
//	for _, tc := range tests {
//		got, _ := TCPPortFromURL(&tc.input)
//		if got != want {
//			t.Fatalf("expected: %v, got: %v", tc.want, got)
//		}
//	}
//
//	testURL := url.URL{Scheme: "http", Host: "example.com:50"}
//
//	want := "50"
//
//	got, err := TcpPortFromURL(testURL)
//	if err != nil {
//		t.Errorf("TcpPortFromUrl(%s) error; %v", err)
//	}
//
//	if got != want {
//		t.Errorf("TcpPortFromUrl(%s) = %d; want %d", testURL.String(), got, want)
//	}
//}
//
//
//
//testURL := url.Parse("http://example.com")
//
//want := 80
//
//got := Abs(-1)
//if got != 1 {
//t.Errorf("Abs(-1) = %d; want 1", got)
//}
//}
