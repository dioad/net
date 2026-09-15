package tlsrpt

import (
	"strings"
	"testing"
)

func TestRecordLongValueIsNotSilentlyTruncated(t *testing.T) {
	uris := make([]string, 0, 15)
	for range 15 {
		uris = append(uris, "https://tls-reports-subdomain-with-a-long-name.example.com/report")
	}
	r := Record{Version: "TLSRPTv1", ReportURIAggregate: uris}

	value := r.String()
	if len(value) <= 255 {
		t.Fatalf("test setup: record value is only %d bytes, need > 255 to exercise chunking", len(value))
	}

	recordValue := r.RecordValue()
	if !strings.Contains(recordValue, `\" \"`) {
		t.Errorf("expected RecordValue to split the value into multiple quoted segments, got: %s", recordValue)
	}

	rejoined := strings.ReplaceAll(strings.ReplaceAll(recordValue, `\"`, ""), " ", "")
	want := strings.ReplaceAll(value, " ", "")
	if rejoined != want {
		t.Errorf("RecordValue lost content: got %q, want (with spaces removed) %q", rejoined, want)
	}
}

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
