// Package dmarc provides types for building a DMARC DNS TXT record.
package dmarc

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/dioad/net/smtp/internal/txtchunk"
)

/*
Should potentially reuse "github.com/emersion/go-msgauth/dmarc" `Record` struct
and rather than repeating it here simply make a way to encode it into a string.
*/

// DMARC policies, per RFC 7489 section 6.3.
const (
	PolicyNone       Policy = "none"
	PolicyQuarantine Policy = "quarantine"
	PolicyReject     Policy = "reject"
)

// DMARC identifier alignment modes, per RFC 7489 section 6.3.
const (
	AlignmentPolicyStrict  AlignmentPolicy = "s"
	AlignmentPolicyRelaxed AlignmentPolicy = "r"
)

// Policy represents a DMARC policy (none, quarantine, reject).
type Policy string

// AlignmentPolicy represents a DMARC alignment policy (strict, relaxed).
type AlignmentPolicy string

// Record represents a DMARC DNS record.
type Record struct {
	Version             string          `mapstructure:"version"`
	Policy              Policy          `mapstructure:"policy"`
	Percent             *uint8          `mapstructure:"percent"`
	ReportURIAggregate  []string        `mapstructure:"report-uri-aggregate"`
	ReportURIFailure    []string        `mapstructure:"report-uri-failure"`
	SubdomainPolicy     Policy          `mapstructure:"subdomain-policy"`
	AlignmentPolicyDKIM AlignmentPolicy `mapstructure:"alignment-policy-dkim"`
	AlignmentPolicySPF  AlignmentPolicy `mapstructure:"alignment-policy-spf"`
}

func formatDMARCEmails(label string, emails []string) string {
	if len(emails) == 0 {
		return ""
	}

	addrs := make([]string, 0, len(emails))
	for _, a := range emails {
		addrs = append(addrs, "mailto:"+a)
	}

	return fmt.Sprintf("%s=%s", label, strings.Join(addrs, ","))
}

// UnsetPercent clears Percent, omitting "pct=" from the rendered record.
func (r *Record) UnsetPercent() {
	r.Percent = nil
}

// SetPercent sets Percent to pct, returning an error if pct exceeds 100.
func (r *Record) SetPercent(pct uint8) error {
	if pct > 100 {
		return errors.New("pct must be between 0 and 100")
	}
	r.Percent = &pct

	return nil
}

func renderList(values []string, data any) ([]string, error) {
	if values == nil {
		return nil, nil
	}

	ret := make([]string, len(values))

	for i := range values {
		tmpl, err := template.New("dmarc").Parse(values[i])
		if err != nil {
			return nil, err
		}
		buf := &bytes.Buffer{}
		err = tmpl.Execute(buf, data)
		if err != nil {
			return nil, err
		}
		ret[i] = buf.String()
	}

	return ret, nil
}

// Render expands the Go template strings in ReportURIAggregate and
// ReportURIFailure against data.
func (r *Record) Render(data any) error {
	var err error

	r.ReportURIAggregate, err = renderList(r.ReportURIAggregate, data)
	if err != nil {
		return err
	}

	r.ReportURIFailure, err = renderList(r.ReportURIFailure, data)
	if err != nil {
		return err
	}

	return nil
}

// RecordType returns the DNS record type string, "TXT".
func (r *Record) RecordType() string {
	return "TXT"
}

// RecordPrefix returns the DNS owner label prefix for a DMARC record,
// always "_dmarc.".
func (r *Record) RecordPrefix() string {
	return "_dmarc."
}

// RecordValue returns the wire-ready, quoted DMARC record value.
func (r *Record) RecordValue() string {
	return txtchunk.Quote(r.String())
}

func (r *Record) String() string {
	parts := make([]string, 0)

	if r.Version == "" {
		parts = append(parts, "v=DMARC1")
	} else {
		parts = append(parts, "v="+r.Version)
	}

	parts = append(parts, fmt.Sprintf("p=%s", r.Policy))

	if r.Percent != nil {
		parts = append(parts, fmt.Sprintf("pct=%d", *r.Percent))
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

	return strings.Join(parts, "; ")
}
