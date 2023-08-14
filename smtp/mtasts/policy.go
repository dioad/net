package mtasts

import (
	"fmt"
	"strings"
)

type Mode string

const (
	ModeNone    Mode = "none"
	ModeTesting Mode = "testing"
	ModeEnforce Mode = "enforce"
)

type Policy struct {
	Version string
	Mode    Mode
	MX      []string
	MaxAge  uint32
}

// FormatPolicy https://www.mailhardener.com/kb/mta-sts
// TODO: use text/template for this stuff?
func FormatPolicy(p *Policy) (string, error) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("version: %s\n", p.Version))
	sb.WriteString(fmt.Sprintf("mode: %s\n", p.Mode))

	for _, v := range p.MX {
		sb.WriteString(fmt.Sprintf("mx: %s\n", v))
	}

	sb.WriteString(fmt.Sprintf("max_age: %d\n", p.MaxAge))

	return sb.String(), nil
}
