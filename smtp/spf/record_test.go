package spf

import "testing"

func TestIPMechanism(t *testing.T) {
	tests := []struct {
		m []string
		r string
	}{
		{
			[]string{"1.2.3.4"},
			"ip4:1.2.3.4",
		},
		{
			[]string{"fdaa:a:f326:a7b:326:fe9f:445d:2"},
			"ip6:fdaa:a:f326:a7b:326:fe9f:445d:2",
		},
		{
			[]string{"1.2.3.4", "fdaa:a:f326:a7b:326:fe9f:445d:2"},
			"ip4:1.2.3.4 ip6:fdaa:a:f326:a7b:326:fe9f:445d:2",
		},
		{
			[]string{"1.2.3.4", "1.3.4.5"},
			"ip4:1.2.3.4 ip4:1.3.4.5",
		},
		{
			[]string{},
			"",
		},
	}

	for _, run := range tests {
		expectedResult := run.r
		var result string

		ip4m, ip6m := IPMechanisms(run.m...)

		if ip4m != nil && ip6m != nil {
			result = FormatMechanisms(*ip4m, *ip6m)
		} else {
			if ip6m != nil {
				result = FormatMechanism(*ip6m)
			}
			if ip4m != nil {
				result = FormatMechanism(*ip4m)
			}
		}

		if result != expectedResult {
			t.Errorf("got: %s, expected: %s", result, expectedResult)
		}
	}
}

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
		{
			Record{
				Version: "spf1",
				All:     true,
				Mechanisms: []Mechanism{
					{
						Name:      "ip4",
						ValueList: "1.4.1.4,1.2.1.2",
					},
					AMechanism("mx.example.com"),
				},
				AllQualifier: QualifierFail,
			},
			"v=spf1 ip4:1.4.1.4 ip4:1.2.1.2 a:mx.example.com -all",
		},
	}

	for _, run := range tests {
		run.s.Render(nil)
		result := run.s.String()
		if result != run.r {
			t.Errorf("got: %s, expected: %s", result, run.r)
		}
	}
}

func TestSPFRecordValueListWithInterpolation(t *testing.T) {
	tests := []struct {
		s Record
		r string
	}{
		{
			Record{
				Version: "spf1",
				All:     true,
				Mechanisms: []Mechanism{
					{
						Name:      "ip4",
						ValueList: "{{ .IPs }}",
					},
					AMechanism("mx.example.com"),
				},
				AllQualifier: QualifierFail,
			},
			"v=spf1 ip4:1.4.1.4 ip4:1.2.1.2 a:mx.example.com -all",
		},
	}

	data := struct {
		IPs string
	}{
		IPs: "1.4.1.4,1.2.1.2",
	}

	for _, run := range tests {
		err := run.s.Render(data)
		if err != nil {
			t.Errorf("got: %v, expected: %v", err, nil)
		}
		result := run.s.String()
		if result != run.r {
			t.Errorf("got: %s, expected: %s", result, run.r)
		}
	}
}

func TestSPFRecordValueFuncWithInterpolation(t *testing.T) {
	valueFunc := func() []string {
		return []string{
			"1.4.1.4",
			"1.2.1.2",
		}
	}

	tests := []struct {
		s Record
		r string
	}{
		{
			Record{
				Version: "spf1",
				All:     true,
				Mechanisms: []Mechanism{
					{
						Name:       "ip4",
						ValuesFunc: valueFunc,
					},
					AMechanism("mx.example.com"),
				},
				AllQualifier: QualifierFail,
			},
			"v=spf1 ip4:1.4.1.4 ip4:1.2.1.2 a:mx.example.com -all",
		},
	}

	// data := struct {
	//	IPs string
	// }{
	//	IPs: "1.4.1.4,1.2.1.2",
	// }

	for _, run := range tests {
		err := run.s.Render(nil)
		if err != nil {
			t.Errorf("got: %v, expected: %v", err, nil)
		}
		result := run.s.String()
		if result != run.r {
			t.Errorf("got: %s, expected: %s", result, run.r)
		}
	}
}

func TestSPFRecordWithMultipleValuesWithInterpolation(t *testing.T) {
	valueFunc := func() []string {
		return []string{
			"1.2.1.2",
			"1.3.1.3",
		}
	}

	tests := []struct {
		s Record
		r string
	}{
		{
			Record{
				Version: "spf1",
				All:     true,
				Mechanisms: []Mechanism{
					{
						Name:       "ip4",
						ValuesFunc: valueFunc,
						ValueList:  "{{ .IPs }}",
					},
					AMechanism("mx.example.com"),
				},
				AllQualifier: QualifierFail,
			},
			"v=spf1 ip4:1.4.1.4 ip4:1.5.1.5 ip4:1.2.1.2 ip4:1.3.1.3 a:mx.example.com -all",
		},
	}

	data := struct {
		IPs string
	}{
		IPs: "1.4.1.4,1.5.1.5",
	}

	for _, run := range tests {
		err := run.s.Render(data)
		if err != nil {
			t.Errorf("got: %v, expected: %v", err, nil)
		}
		result := run.s.String()
		if result != run.r {
			t.Errorf("got: %s, expected: %s", result, run.r)
		}
	}
}
