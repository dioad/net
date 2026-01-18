package mtasts

import (
	"fmt"
	"strings"
)

// Record represents an MTA-STS DNS TXT record.
type Record struct {
	Version string `mapstructure:"version"`
	ID      string `mapstructure:"id"`
}

func (r *Record) RecordPrefix() string {
	return "_mta-sts."
}

func (r *Record) RecordType() string {
	return "TXT"
}

func (r *Record) RecordValue() string {
	return fmt.Sprintf("\\\"%v\\\"", r.String())
}

func (r *Record) String() string {
	var sb strings.Builder

	if r.Version == "" {
		sb.WriteString("v=STSv1; ")
	} else {
		sb.WriteString(fmt.Sprintf("v=%s ", r.Version))
	}
	sb.WriteString(fmt.Sprintf("id=%s", r.ID))

	result := sb.String()
	if len(result) > 255 {
		result = result[:255]
	}
	return result
}
