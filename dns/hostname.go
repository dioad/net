package dns

import (
	"errors"
	"fmt"
	"strings"
)

const (
	maxHostnameLength = 253
	maxLabelLength    = 63
)

// ErrInvalidHostname is wrapped by every error ValidateHostname returns.
var ErrInvalidHostname = errors.New("invalid hostname")

// ValidateHostname returns an error wrapping ErrInvalidHostname if s is not a
// valid DNS hostname.
//
// A hostname is one or more dot-separated labels. Each label is 1-63
// characters, contains only ASCII letters, digits, underscores and hyphens,
// and does not begin or end with a hyphen. The total length, excluding one
// optional trailing dot, is 1-253 characters, and an empty label (as in
// "a..b") is rejected.
//
// Underscores are accepted deliberately: names such as
// "_acme-challenge.example.com", "_dmarc.example.com" and
// "_smtp._tls.example.com" need them. Letters of either case are accepted,
// since DNS names are case-insensitive.
func ValidateHostname(s string) error {
	name := strings.TrimSuffix(s, ".")
	if len(name) == 0 || len(name) > maxHostnameLength {
		return fmt.Errorf("%w: %q has invalid length (must be 1-%d characters)", ErrInvalidHostname, s, maxHostnameLength)
	}

	for label := range strings.SplitSeq(name, ".") {
		if err := validateLabel(s, label); err != nil {
			return err
		}
	}

	return nil
}

func validateLabel(hostname, label string) error {
	if len(label) == 0 {
		return fmt.Errorf("%w: %q contains an empty label", ErrInvalidHostname, hostname)
	}
	if len(label) > maxLabelLength {
		return fmt.Errorf("%w: %q has a label exceeding %d characters", ErrInvalidHostname, hostname, maxLabelLength)
	}
	if label[0] == '-' || label[len(label)-1] == '-' {
		return fmt.Errorf("%w: %q has a label that begins or ends with a hyphen", ErrInvalidHostname, hostname)
	}
	for _, c := range label {
		if !isLabelChar(c) {
			return fmt.Errorf("%w: %q contains invalid character %q", ErrInvalidHostname, hostname, c)
		}
	}

	return nil
}

// isLabelChar reports whether c is an ASCII letter, digit, hyphen or underscore.
func isLabelChar(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_'
}
