package txtchunk

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuote(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		assert.Equal(t, `\"\"`, Quote(""))
	})

	t.Run("short string fits in one segment", func(t *testing.T) {
		assert.Equal(t, `\"v=spf1 -all\"`, Quote("v=spf1 -all"))
	})

	t.Run("string over 255 bytes splits into multiple quoted segments", func(t *testing.T) {
		s := strings.Repeat("a", 300)
		got := Quote(s)

		want := `\"` + strings.Repeat("a", 255) + `\" \"` + strings.Repeat("a", 45) + `\"`
		assert.Equal(t, want, got)

		// No content is dropped: every byte of the input reappears in the output.
		assert.Equal(t, len(s), strings.Count(got, "a"))
	})

	t.Run("string that is an exact multiple of 255 bytes does not emit a trailing empty segment", func(t *testing.T) {
		s := strings.Repeat("b", 510)
		got := Quote(s)

		assert.Equal(t, 2, strings.Count(got, `\"b`))
	})
}
