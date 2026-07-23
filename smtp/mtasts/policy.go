package mtasts

import (
	"fmt"
	"strings"
)

// Mode represents an MTA-STS mode (none, testing, enforce).
type Mode string

const (
	ModeNone    Mode = "none"
	ModeTesting Mode = "testing"
	ModeEnforce Mode = "enforce"
)

// Policy represents an MTA-STS policy.
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

	fmt.Fprintf(&sb, "version: %s\n", p.Version)
	fmt.Fprintf(&sb, "mode: %s\n", p.Mode)

	for _, v := range p.MX {
		fmt.Fprintf(&sb, "mx: %s\n", v)
	}

	fmt.Fprintf(&sb, "max_age: %d\n", p.MaxAge)

	return sb.String(), nil
}
