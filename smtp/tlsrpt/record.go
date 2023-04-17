package tlsrpt

import (
	"fmt"
	"strings"
)

type Record struct {
	Version            string   `mapstructure:"version"`
	ReportURIAggregate []string `mapstructure:"report-uri-aggregate"`
}

func formatRUA(label string, locations []string) string {
	if len(locations) == 0 {
		return ""
	}

	addrs := make([]string, 0, len(locations))
	for _, a := range locations {
		if strings.HasPrefix(a, "https://") {
			// Need to encode a https://www.rfc-editor.org/rfc/rfc8460#section-3
			addrs = append(addrs, a)
		} else {
			addrs = append(addrs, fmt.Sprintf("mailto:%s", a))
		}
	}

	return fmt.Sprintf("%s=%s", label, strings.Join(addrs, ","))
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
