package dmarc

import (
	"testing"

	"github.com/emersion/go-msgauth/dmarc"
)

func TestFormatDMARCEmails(t *testing.T) {
	label := "rua"
	tests := []struct {
		m []string
		r string
	}{
		{
			m: []string{},
			r: "",
		},
		{
			m: []string{"abcde@example.com"},
			r: "rua=mailto:abcde@example.com",
		},
		{
			m: []string{"abcde@example.com", "other@abcd.com"},
			r: "rua=mailto:abcde@example.com,mailto:other@abcd.com",
		},
	}

	for _, run := range tests {
		result := formatDMARCEmails(label, run.m)
		if result != run.r {
			t.Errorf("got: %s, expected: %s", result, run.r)
		}
	}
}

func TestRecordPercentInBounds(t *testing.T) {
	r := Record{
		Version: "DMARC1",
		Policy:  PolicyQuarantine,
	}
	r.SetPercent(45)

	result := r.String()

	expected := "v=DMARC1; p=quarantine; pct=45"

	if result != expected {
		t.Errorf("got: %s, expected: %s", result, expected)
	}
}

func TestDMARCRecord(t *testing.T) {
	tests := []struct {
		s Record
		r string
	}{
		{
			Record{
				Version: "DMARC1",
				Policy:  PolicyReject,
			},
			"v=DMARC1; p=reject",
		},
		{
			Record{
				Version:            "DMARC1",
				Policy:             PolicyNone,
				ReportURIAggregate: []string{"ex1@mple.com"},
			},
			"v=DMARC1; p=none; rua=mailto:ex1@mple.com",
		},
		{
			Record{
				Version:            "DMARC1",
				Policy:             PolicyNone,
				ReportURIAggregate: []string{"ex2@mple.com"},
			},
			"v=DMARC1; p=none; rua=mailto:ex2@mple.com",
		},
		{
			Record{
				Version:            "DMARC1",
				Policy:             PolicyNone,
				ReportURIAggregate: []string{"ex2@mple.com"},
				ReportURIFailure:   []string{"failed@example.com"},
			},
			"v=DMARC1; p=none; rua=mailto:ex2@mple.com; ruf=mailto:failed@example.com",
		},
		{
			Record{
				Version:            "DMARC1",
				Policy:             PolicyNone,
				ReportURIAggregate: []string{"ex2@mple.com"},
				ReportURIFailure:   []string{"failed@example.com"},
				SubdomainPolicy:    PolicyReject,
			},
			"v=DMARC1; p=none; sp=reject; rua=mailto:ex2@mple.com; ruf=mailto:failed@example.com",
		},
		{
			Record{
				Version:             "DMARC1",
				Policy:              PolicyNone,
				ReportURIAggregate:  []string{"ex2@mple.com"},
				ReportURIFailure:    []string{"failed@example.com"},
				SubdomainPolicy:     PolicyReject,
				AlignmentPolicyDKIM: AlignmentPolicyRelaxed,
			},
			"v=DMARC1; p=none; sp=reject; rua=mailto:ex2@mple.com; ruf=mailto:failed@example.com; adkim=r",
		},
		{
			Record{
				Version:            "DMARC1",
				Policy:             PolicyNone,
				ReportURIAggregate: []string{"ex2@mple.com"},
				ReportURIFailure:   []string{"failed@example.com"},
				SubdomainPolicy:    PolicyReject,
				AlignmentPolicySPF: AlignmentPolicyStrict,
			},
			"v=DMARC1; p=none; sp=reject; rua=mailto:ex2@mple.com; ruf=mailto:failed@example.com; aspf=s",
		},
		{
			Record{
				Version:             "DMARC1",
				Policy:              PolicyNone,
				ReportURIAggregate:  []string{"ex2@mple.com"},
				ReportURIFailure:    []string{"failed@example.com"},
				SubdomainPolicy:     PolicyReject,
				AlignmentPolicyDKIM: AlignmentPolicyRelaxed,
				AlignmentPolicySPF:  AlignmentPolicyStrict,
			},
			"v=DMARC1; p=none; sp=reject; rua=mailto:ex2@mple.com; ruf=mailto:failed@example.com; adkim=r; aspf=s",
		},
	}

	for id, run := range tests {
		result := run.s.String()
		if result != run.r {
			t.Errorf("(%d) got: %s, expected: %s", id, result, run.r)
		}
		_, err := dmarc.Parse(result)
		if err != nil {
			t.Errorf("(%d) failed to parse dmarc: %s", id, err)
		}
	}
}
