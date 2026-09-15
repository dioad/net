// Package txtchunk renders DNS TXT record values as zone-file text.
//
// A single DNS character-string is limited to 255 octets (RFC 1035 §3.3).
// A TXT record value longer than that must be split across multiple
// quoted character-strings on the same answer line; zone-file parsers
// concatenate them back into one value when the record is loaded.
package txtchunk

import "fmt"

const maxCharString = 255

// Quote splits s into 255-byte character-strings and renders each as an
// escaped, double-quoted segment, space-separated, ready for insertion into
// a zone-file TXT record answer line.
func Quote(s string) string {
	if s == "" {
		return `\"\"`
	}

	var out string
	for len(s) > 0 {
		n := min(maxCharString, len(s))
		if out != "" {
			out += " "
		}
		out += fmt.Sprintf(`\"%s\"`, s[:n])
		s = s[n:]
	}
	return out
}
