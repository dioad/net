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

func TestRecord_EncodesCommaAndExclamationInURI(t *testing.T) {
	r := Record{
		Version:            "TLSRPTv1",
		ReportURIAggregate: []string{"https://example.com/report?a=1,2!important", "ops@example.com"},
	}

	value := r.String()

	_, ruaField, found := strings.Cut(value, "rua=")
	if !found {
		t.Fatalf("rua field not found in %q", value)
	}

	// A comma or exclamation point occurring inside a URI, once percent-
	// encoded, must not be mistaken for the rua list's own "," delimiter --
	// splitting on "," must recover exactly the two configured URIs.
	uris := strings.Split(ruaField, ",")
	if len(uris) != 2 {
		t.Fatalf("comma/exclamation inside a URI was not escaped: rua field split into %d entries, want 2: %v (raw: %q)", len(uris), uris, ruaField)
	}
	if strings.Contains(uris[0], ",") || strings.Contains(uris[0], "!") {
		t.Errorf("first URI still contains an unescaped ',' or '!': %q", uris[0])
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
