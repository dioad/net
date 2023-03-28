package spf

import "testing"

func TestFormatMechanism(t *testing.T) {
	tests := []struct {
		m Mechanism
		r string
	}{
		{
			IP4Mechanism("1.2.3.4"),
			"ip4:1.2.3.4",
		},
		{
			IP4Mechanism("1.2.3.4", "1.3.4.5"),
			"ip4:1.2.3.4 ip4:1.3.4.5",
		},
		{
			IP4Mechanism(),
			"",
		},
	}

	for _, run := range tests {
		result := FormatMechanism(run.m)
		if result != run.r {
			t.Errorf("got: %s, expected: %s", result, run.r)
		}
	}
}

func TestSPFRecord(t *testing.T) {
	tests := []struct {
		s Record
		r string
	}{
		{
			Record{
				Version: "spf1",
			},
			"v=spf1",
		},
		{
			Record{
				Version: "spf1",
				All:     true,
			},
			"v=spf1 all",
		},
		{
			Record{
				Version:      "spf1",
				All:          true,
				AllQualifier: QualifierFail,
			},
			"v=spf1 -all",
		},
		{
			Record{
				Version: "spf1",
				All:     true,
				Mechanisms: []Mechanism{
					IP4Mechanism("1.4.1.4"),
				},
				AllQualifier: QualifierFail,
			},
			"v=spf1 ip4:1.4.1.4 -all",
		},
		{
			Record{
				Version: "spf1",
				All:     true,
				Mechanisms: []Mechanism{
					IP4Mechanism("1.4.1.4"),
					AMechanism("mx.example.com"),
				},
				AllQualifier: QualifierFail,
			},
			"v=spf1 ip4:1.4.1.4 a:mx.example.com -all",
		},
	}

	for _, run := range tests {
		result := run.s.String()
		if result != run.r {
			t.Errorf("got: %s, expected: %s", result, run.r)
		}
	}
}
