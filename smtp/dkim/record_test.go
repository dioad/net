package dkim

import (
	"strings"
	"testing"
)

func TestDKIMRecord(t *testing.T) {
	r := Record{
		Version:   "DKIM1",
		KeyType:   "rsa",
		PublicKey: "EXAMPLE=",
	}

	expected := "v=DKIM1; k=rsa; p=EXAMPLE="

	result := r.String()

	if result != expected {
		t.Errorf("got: %s, expected %s", result, expected)
	}
}

func TestParseParam(t *testing.T) {
	input := "v=DKIM1; k=rsa; p=EXAMPLE="

	params, err := ParseParams(input)
	if err != nil {
		t.Fatalf("failed to parse params: %v", err)
	}
	if params["v"] != "DKIM1" {
		t.Errorf("params[v]: got: %s, expected: %s", params["v"], "DKIM1")
	}
	if params["p"] != "EXAMPLE=" {
		t.Errorf("params[p]: got: %s, expected: %s", params["p"], "EXAMPLE=")
	}
	if params["k"] != "rsa" {
		t.Errorf("params[k]: got: %s, expected: %s", params["k"], "rsa")
	}
}

func TestRecordFileReader(t *testing.T) {
	reader := strings.NewReader("v=DKIM1; k=rsa; p=EXAMPLE=")
	r, err := FromRecordFile(reader)

	if err != nil {
		t.Errorf("failed to load record: %s", err)
	}

	if r.Version != "DKIM1" {
		t.Errorf("got: %s, expected: %s", r.Version, "DKIM1")
	}

	if r.KeyType != "rsa" {
		t.Errorf("got: %s, expected: %s", r.KeyType, "rsa")
	}

	if r.PublicKey != "EXAMPLE=" {
		t.Errorf("got: %s, expected: %s", r.PublicKey, "EXAMPLE=")
	}
}
