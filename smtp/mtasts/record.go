package mtasts

import (
	"fmt"
	"strings"

	"github.com/dioad/net/smtp/internal/txtchunk"
)

// Record represents an MTA-STS DNS TXT record.
type Record struct {
	Version string `mapstructure:"version"`
	ID      string `mapstructure:"id"`
}

// RecordPrefix returns the DNS owner label prefix for an MTA-STS record,
// always "_mta-sts.".
func (r *Record) RecordPrefix() string {
	return "_mta-sts."
}

// RecordType returns the DNS record type string, "TXT".
func (r *Record) RecordType() string {
	return "TXT"
}

// RecordValue returns the wire-ready, quoted MTA-STS record value.
func (r *Record) RecordValue() string {
	return txtchunk.Quote(r.String())
}

func (r *Record) String() string {
	version := r.Version
	if version == "" {
		version = "STSv1"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "v=%s; ", version)
	fmt.Fprintf(&sb, "id=%s", r.ID)

	return sb.String()
}
