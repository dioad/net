package dns_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dioad/net/dns"
)

func TestTXTRecord_RecordPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rec      dns.TXTRecord
		expected string
	}{
		{"apex (empty name)", dns.TXTRecord{Value: "v=spf1 -all"}, ""},
		{"subdomain label", dns.TXTRecord{Name: "_dmarc", Value: "v=DMARC1"}, "_dmarc."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, tc.rec.RecordPrefix())
		})
	}
}

func TestTXTRecord_RecordType(t *testing.T) {
	t.Parallel()
	rec := dns.TXTRecord{Value: "hello"}
	assert.Equal(t, "TXT", rec.RecordType())
}

func TestTXTRecord_RecordValue(t *testing.T) {
	t.Parallel()
	rec := dns.TXTRecord{Value: "v=spf1 mx -all"}
	assert.Equal(t, `\"v=spf1 mx -all\"`, rec.RecordValue())
}

func TestTXTRecord_RecordTTL(t *testing.T) {
	t.Parallel()

	t.Run("uses default when zero", func(t *testing.T) {
		t.Parallel()
		rec := dns.TXTRecord{Value: "x"}
		assert.Equal(t, dns.DefaultTTL, rec.RecordTTL())
	})

	t.Run("returns configured TTL", func(t *testing.T) {
		t.Parallel()
		rec := dns.TXTRecord{Value: "x", TTL: 300}
		assert.Equal(t, uint32(300), rec.RecordTTL())
	})
}

func TestTXTRecord_Empty(t *testing.T) {
	t.Parallel()
	assert.True(t, (&dns.TXTRecord{}).Empty())
	assert.False(t, (&dns.TXTRecord{Value: "x"}).Empty())
}

func TestTXTRecord_Render(t *testing.T) {
	t.Parallel()

	type renderData struct {
		Email  string
		Domain string
	}

	t.Run("expands template variables", func(t *testing.T) {
		t.Parallel()
		rec := dns.TXTRecord{Value: "rua=mailto:{{.Email}}"}
		err := rec.Render(&renderData{Email: "dmarc@example.com"})
		require.NoError(t, err)
		assert.Equal(t, "rua=mailto:dmarc@example.com", rec.Value)
	})

	t.Run("static value unchanged", func(t *testing.T) {
		t.Parallel()
		rec := dns.TXTRecord{Value: "v=spf1 mx -all"}
		err := rec.Render(&renderData{})
		require.NoError(t, err)
		assert.Equal(t, "v=spf1 mx -all", rec.Value)
	})

	t.Run("invalid template returns error", func(t *testing.T) {
		t.Parallel()
		rec := dns.TXTRecord{Value: "{{.Unclosed"}
		err := rec.Render(&renderData{})
		assert.Error(t, err)
	})
}
