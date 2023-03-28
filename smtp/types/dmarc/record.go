package dmarc

import (
	"errors"
	"fmt"
	"strings"
)

/*
Should potentially reuse "github.com/emersion/go-msgauth/dmarc" `Record` struct
and rather than repeating it here simply make a way to encode it into a string.
*/

const (
	PolicyNone       Policy = "none"
	PolicyQuarantine        = "quarantine"
	PolicyReject            = "reject"
)

const (
	AlignmentPolicyStrict  AlignmentPolicy = "s"
	AlignmentPolicyRelaxed                 = "r"
)

type Policy string

type AlignmentPolicy string

type Record struct {
	Version             string
	Policy              Policy
	percent             *uint8
	ReportURIAggregate  []string
	ReportURIFailure    []string
	SubdomainPolicy     Policy
	AlignmentPolicyDKIM AlignmentPolicy
	AlignmentPolicySPF  AlignmentPolicy
}

func formatDMARCEmails(label string, emails []string) string {
	if len(emails) == 0 {
		return ""
	}

	addrs := make([]string, 0, len(emails))
	for _, a := range emails {
		addrs = append(addrs, fmt.Sprintf("mailto:%s", a))
	}

	return fmt.Sprintf("%s=%s", label, strings.Join(addrs, ","))
}

func (r *Record) UnsetPercent() {
	r.percent = nil
}

func (r *Record) SetPercent(pct uint8) error {
	if pct > 100 {
		return errors.New("pct must be between 0 and 100")
	}
	r.percent = &pct
	return nil
}

func (r *Record) String() string {
	parts := make([]string, 0)

	parts = append(parts, fmt.Sprintf("v=%s", r.Version))

	parts = append(parts, fmt.Sprintf("p=%s", r.Policy))

	if r.percent != nil {
		parts = append(parts, fmt.Sprintf("pct=%d", *r.percent))
	}

	if r.SubdomainPolicy != "" {
		parts = append(parts, fmt.Sprintf("sp=%s", r.SubdomainPolicy))
	}

	if len(r.ReportURIAggregate) > 0 {
		parts = append(parts, formatDMARCEmails("rua", r.ReportURIAggregate))
	}

	if len(r.ReportURIFailure) > 0 {
		parts = append(parts, formatDMARCEmails("ruf", r.ReportURIFailure))
	}

	if r.AlignmentPolicyDKIM != "" {
		parts = append(parts, fmt.Sprintf("adkim=%s", r.AlignmentPolicyDKIM))
	}

	if r.AlignmentPolicySPF != "" {
		parts = append(parts, fmt.Sprintf("aspf=%s", r.AlignmentPolicySPF))
	}

	result := strings.Join(parts, "; ")
	if len(result) > 255 {
		panic(fmt.Sprintf("too many chars for Record record: %s", result))
	}
	return result
}
