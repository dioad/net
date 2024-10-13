package dmarc

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"text/template"
)

/*
Should potentially reuse "github.com/emersion/go-msgauth/dmarc" `Record` struct
and rather than repeating it here simply make a way to encode it into a string.
*/

const (
	PolicyNone       Policy = "none"
	PolicyQuarantine Policy = "quarantine"
	PolicyReject     Policy = "reject"
)

const (
	AlignmentPolicyStrict  AlignmentPolicy = "s"
	AlignmentPolicyRelaxed AlignmentPolicy = "r"
)

type Policy string

type AlignmentPolicy string

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
		addrs = append(addrs, fmt.Sprintf("mailto:%s", a))
	}

	return fmt.Sprintf("%s=%s", label, strings.Join(addrs, ","))
}

func (r *Record) UnsetPercent() {
	r.Percent = nil
}

func (r *Record) SetPercent(pct uint8) error {
	if pct > 100 {
		return errors.New("pct must be between 0 and 100")
	}
	r.Percent = &pct
	return nil
}

func renderList(values []string, data interface{}) ([]string, error) {
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

func (r *Record) Render(data interface{}) error {
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

func (r *Record) RecordType() string {
	return "TXT"
}

func (r *Record) RecordPrefix() string {
	return "_dmarc."
}

func (r *Record) RecordValue() string {
	return fmt.Sprintf("\\\"%v\\\"", r.String())
}

func (r *Record) String() string {
	parts := make([]string, 0)

	if r.Version == "" {
		parts = append(parts, "v=DMARC1")
	} else {
		parts = append(parts, fmt.Sprintf("v=%s", r.Version))
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

	result := strings.Join(parts, "; ")
	if len(result) > 255 {
		result = result[:255]
	}
	return result
}
