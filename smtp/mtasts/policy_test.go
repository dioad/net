package mtasts

import (
	"testing"
)

func TestSimplePolicy(t *testing.T) {
	p := Policy{
		Version: "STSv1",
		Mode:    ModeTesting,
		MX:      []string{"mx.example.com"},
		MaxAge:  3600,
	}

	expected := "version: STSv1\nmode: testing\nmx: mx.example.com\nmax_age: 3600\n"

	result, _ := FormatPolicy(&p)

	if expected != result {
		t.Errorf("got: %s, expected: %s", result, expected)
	}
}
