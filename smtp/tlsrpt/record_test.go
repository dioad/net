package tlsrpt

import "testing"

func TestRecord(t *testing.T) {
	r := Record{
		Version:            "TLSRPTv1",
		ReportURIAggregate: []string{"blah@example.org", "https://example.com"},
	}

	if r.RecordPrefix() != "_smtp._tls." {
		t.Errorf("RecordPrefix: expected %s, got %s", "_smtp._tls.", r.RecordPrefix())
	}

	if r.RecordType() != "TXT" {
		t.Errorf("RecordType: expected %s, got %s", "TXT", r.RecordType())
	}

	expectedValue := "\\\"v=TLSRPTv1;rua=mailto:blah@example.org,https://example.com\\\""
	if r.RecordValue() != expectedValue {
		t.Errorf("RecordValue: expected %s, got %s", expectedValue, r.RecordValue())
	}

}
