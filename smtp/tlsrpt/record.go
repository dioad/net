package tlsrpt

import (
	"fmt"
	"strings"

	"github.com/dioad/generics"

	"github.com/dioad/net/smtp/internal/txtchunk"
)

// Record represents a TLSRPT (TLS Reporting) DNS TXT record.
type Record struct {
	Version            string   `mapstructure:"version"`
	ReportURIAggregate []string `mapstructure:"report-uri-aggregate"`
}

// encodeRUAURI percent-encodes the characters RFC 8460 3 requires to be
// escaped within a "rua" URI -- "," and "!" -- since left as-is they would
// be indistinguishable from the field's own comma-separated list syntax.
func encodeRUAURI(uri string) string {
	uri = strings.ReplaceAll(uri, "!", "%21")
	uri = strings.ReplaceAll(uri, ",", "%2C")
	return uri
}

func formatRUA(label string, locations []string) string {
	if len(locations) == 0 {
		return ""
	}

	addrs := generics.SafeMap(func(a string) string {
		if strings.HasPrefix(a, "https://") {
			return encodeRUAURI(a)
		}
		return encodeRUAURI(fmt.Sprintf("mailto:%s", a))
	}, locations)

	return fmt.Sprintf("%s=%s", label, strings.Join(addrs, ","))
}

func (r *Record) RecordPrefix() string {
	return "_smtp._tls."
}

func (r *Record) RecordType() string {
	return "TXT"
}

func (r *Record) RecordValue() string {
	return txtchunk.Quote(r.String())
}

func (r *Record) String() string {
	parts := make([]string, 0)
	if r.Version == "" {
		parts = append(parts, "v=TLSRPTv1")
	} else {
		parts = append(parts, fmt.Sprintf("v=%s", r.Version))
	}

	parts = append(parts, formatRUA("rua", r.ReportURIAggregate))

	return strings.Join(parts, ";")
}
