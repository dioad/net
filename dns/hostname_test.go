package dns

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateHostname(t *testing.T) {
	t.Parallel()

	label63 := strings.Repeat("a", 63)
	tests := []struct {
		hostname string
		wantErr  string
	}{
		{"example.com", ""},
		{"sub.example.com", ""},
		{"localhost", ""},
		{"hyphen-ated.com", ""},
		{"UPPER.com", ""},
		{"example.com.", ""},
		{"123.example.com", ""},
		{"_acme-challenge.example.com", ""},
		{"_dmarc.blah.com", ""},
		{"_mta-sts.blah.com", ""},
		{"_smtp._tls.blah.com", ""},
		{"default._domainkey.blah.com", ""},
		{"_25._tcp.blah.com", ""},
		{label63 + ".com", ""},
		{"a", ""},
		{strings.Join([]string{label63, label63, label63, strings.Repeat("a", 61)}, "."), ""},

		{"", "invalid length"},
		{".", "invalid length"},
		{strings.Join([]string{label63, label63, label63, strings.Repeat("a", 62)}, "."), "invalid length"},
		{"-invalid.com", "begins or ends with a hyphen"},
		{"invalid-.com", "begins or ends with a hyphen"},
		{"..invalid.com", "empty label"},
		{"a..b", "empty label"},
		{"example.com..", "empty label"},
		{".example.com", "empty label"},
		{"too-long-" + strings.Repeat("a", 64) + ".com", "exceeding 63 characters"},
		{"exa mple.com", "invalid character"},
		{"exa/mple.com", "invalid character"},
		{"bücher.example", "invalid character"},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			t.Parallel()

			err := ValidateHostname(tt.hostname)

			if tt.wantErr == "" {
				require.NoError(t, err)

				return
			}
			require.ErrorIs(t, err, ErrInvalidHostname)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func ExampleValidateHostname() {
	fmt.Println(ValidateHostname("_acme-challenge.example.com"))
	fmt.Println(ValidateHostname("-bad.example.com"))
	// Output:
	// <nil>
	// invalid hostname: "-bad.example.com" has a label that begins or ends with a hyphen
}
