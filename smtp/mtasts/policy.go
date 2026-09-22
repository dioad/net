package mtasts

import (
	"fmt"
	"strings"
)

// Mode represents an MTA-STS mode (none, testing, enforce).
type Mode string

// MTA-STS modes, per RFC 8461 section 3.
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
	// RFC 8461 3.2: at least one "mx" field MUST appear in a policy record
	// with mode "testing" or "enforce". Mode "none" has no such
	// requirement, since MTA-STS enforcement is disabled either way.
	if (p.Mode == ModeTesting || p.Mode == ModeEnforce) && len(p.MX) == 0 {
		return "", fmt.Errorf("mtasts: mode %q requires at least one mx entry", p.Mode)
	}

	var sb strings.Builder

	fmt.Fprintf(&sb, "version: %s\n", p.Version)
	fmt.Fprintf(&sb, "mode: %s\n", p.Mode)

	for _, v := range p.MX {
		fmt.Fprintf(&sb, "mx: %s\n", v)
	}

	fmt.Fprintf(&sb, "max_age: %d\n", p.MaxAge)

	return sb.String(), nil
}
