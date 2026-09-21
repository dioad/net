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
		{"2001:db8::1", "", true}, // reverse DNS notation is IPv4-only; IPv6 must error, not silently return ""
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
		if got == "" {
			t.Errorf("ReverseIP(%q) returned an empty string with no error", addr)
		}
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

func TestBlockListLookupAddr_IPv6Errors(t *testing.T) {
	// Before this fix, ReverseIP silently returned ("", nil) for IPv6,
	// so BlocklistLookupAddr queried the malformed hostname
	// ".zen.spamhaus.org", which failed DNS resolution and was
	// interpreted as "not listed" -- an IPv6 address could never
	// actually be checked, but no error surfaced either.
	_, err := BlocklistLookupAddr("2001:db8::1")
	if err == nil {
		t.Error("BlocklistLookupAddr on an IPv6 address should return an error, not silently report \"not listed\"")
	}
}
