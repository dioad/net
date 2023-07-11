package dns

import "testing"

func TestUitoa(t *testing.T) {
	var tests = []struct {
		in  uint64
		out string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{10000000000, "10000000000"},
	}
	for _, test := range tests {
		if out := uitoa(test.in); out != test.out {
			t.Errorf("uitoa(%v) = %v", test.in, out)
		}
	}
}

func TestReverseIP(t *testing.T) {
	var tests = []struct {
		in  string
		out string
	}{
		{"127.0.0.1", "1.0.0.127"},
		{"128.3.244.164", "164.244.3.128"},
	}
	for _, test := range tests {
		if out, _ := ReverseIP(test.in); out != test.out {
			t.Errorf("ReverseIP(%v) = %v", test.in, out)
		}
	}
}

func TestBlockListLookupAddr(t *testing.T) {
	var tests = []struct {
		in  string
		out bool
	}{
		{"127.0.0.2", true},
	}
	for _, test := range tests {
		if out, _ := BlocklistLookupAddr(test.in); out != test.out {
			t.Errorf("BlocklistLookupAddr(%v) = %v", test.in, out)
		}
	}
}
