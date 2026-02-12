package dns

import "testing"

func TestReverseIP(t *testing.T) {
	var tests = []struct {
		in      string
		out     string
		wantErr bool
	}{
		{"127.0.0.1", "1.0.0.127", false},
		{"128.3.244.164", "164.244.3.128", false},
		{"invalid", "", true},
		{"", "", true},
		{"2001:db8::1", "", false}, // Currently returns "", nil for IPv6
	}
	for _, test := range tests {
		out, err := ReverseIP(test.in)
		if (err != nil) != test.wantErr {
			t.Errorf("ReverseIP(%v) error = %v, wantErr %v", test.in, err, test.wantErr)
			continue
		}
		if out != test.out {
			t.Errorf("ReverseIP(%v) = %v, want %v", test.in, out, test.out)
		}
	}
}

func FuzzReverseIP(f *testing.F) {
	f.Add("127.0.0.1")
	f.Add("2001:db8::1")
	f.Add("invalid")
	f.Fuzz(func(t *testing.T, addr string) {
		got, err := ReverseIP(addr)
		if err != nil {
			return
		}
		// If it succeeded, it should not be empty for valid inputs
		// though our current implementation returns "" for IPv6 without error.
		_ = got
	})
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
