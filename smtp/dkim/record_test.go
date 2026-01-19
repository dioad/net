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
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid",
			input:   "v=DKIM1; k=rsa; p=EXAMPLE=",
			wantErr: false,
		},
		{
			name:    "extra spaces",
			input:   " v = DKIM1 ;  k = rsa ; p = EXAMPLE= ",
			wantErr: false,
		},
		{
			name:    "invalid parameter",
			input:   "v=DKIM1; x=unknown",
			wantErr: true,
		},
		{
			name:    "missing value",
			input:   "v=DKIM1; k=; p=EXAMPLE=",
			wantErr: false, // parseParams will just have empty string value
		},
		{
			name:    "no equals",
			input:   "v",
			wantErr: true, // "v" is invalid parameter because it doesn't match keys if it's trimmed? wait.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseParams(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseParams() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func FuzzParseParams(f *testing.F) {
	f.Add("v=DKIM1; k=rsa; p=EXAMPLE=")
	f.Add("v=DKIM1; k=; p=")
	f.Add("invalid=param")
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = ParseParams(s)
	})
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
