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

func (r *Record) RecordPrefix() string {
	return "_mta-sts."
}

func (r *Record) RecordType() string {
	return "TXT"
}

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
