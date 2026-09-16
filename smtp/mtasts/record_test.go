package mtasts

import (
	"strings"
	"testing"
)

func TestRecordString_SeparatorConsistentRegardlessOfVersion(t *testing.T) {
	defaultVersion := Record{ID: "x"}
	explicitVersion := Record{Version: "STSv1", ID: "x"}

	if got, want := defaultVersion.String(), "v=STSv1; id=x"; got != want {
		t.Fatalf("test setup: default-Version case = %q, want %q", got, want)
	}

	if got, want := explicitVersion.String(), "v=STSv1; id=x"; got != want {
		t.Errorf("explicit-Version case = %q, want %q (same semantic record as the default-Version case must serialize identically)", got, want)
	}
}

func TestRecordLongValueIsNotSilentlyTruncated(t *testing.T) {
	r := Record{Version: "STSv1", ID: strings.Repeat("2026091500", 30)}

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
