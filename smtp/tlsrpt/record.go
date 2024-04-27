package tlsrpt

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/dioad/generics"
)

type Record struct {
	Version            string   `mapstructure:"version"`
	ReportURIAggregate []string `mapstructure:"report-uri-aggregate"`
}

func (r *Record) Empty() bool {
	return reflect.DeepEqual(*r, Record{})
}

func formatRUA(label string, locations []string) string {
	if len(locations) == 0 {
		return ""
	}

	addrs := generics.SafeMap(func(a string) string {
		if strings.HasPrefix(a, "https://") {
			// Need to encode a https://www.rfc-editor.org/rfc/rfc8460#section-3
			return a
		}
		return fmt.Sprintf("mailto:%s", a)
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
	return fmt.Sprintf("\\\"%v\\\"", r.String())
}

func (r *Record) String() string {
	parts := make([]string, 0)
	if r.Version == "" {
		parts = append(parts, "v=TLSRPTv1")
	} else {
		parts = append(parts, fmt.Sprintf("v=%s", r.Version))
	}

	parts = append(parts, formatRUA("rua", r.ReportURIAggregate))

	result := strings.Join(parts, ";")
	if len(result) > 255 {
		panic(fmt.Sprintf("too many chars for Record record: %s", result))
	}
	return result
}
