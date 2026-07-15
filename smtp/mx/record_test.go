package mx_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dioad/net/dns"
	"github.com/dioad/net/smtp/mx"
)

func TestRecord_RecordPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rec      mx.Record
		expected string
	}{
		{"apex (empty prefix)", mx.Record{Priority: 10, Host: "mail.example.com."}, ""},
		{"sub-label prefix", mx.Record{Priority: 10, Host: "mail.example.com.", Prefix: "smtp"}, "smtp."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, tc.rec.RecordPrefix())
		})
	}
}

func TestRecord_RecordType(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "MX", (&mx.Record{Priority: 10, Host: "mail.example.com."}).RecordType())
}

func TestRecord_RecordValue(t *testing.T) {
	t.Parallel()
	rec := mx.Record{Priority: 10, Host: "mail.example.com."}
	assert.Equal(t, "10 mail.example.com.", rec.RecordValue())
}

func TestRecord_RecordTTL(t *testing.T) {
	t.Parallel()

	t.Run("uses dns.DefaultTTL when zero", func(t *testing.T) {
		t.Parallel()
		rec := mx.Record{Priority: 10, Host: "mail.example.com."}
		assert.Equal(t, dns.DefaultTTL, rec.RecordTTL())
	})

	t.Run("returns configured TTL", func(t *testing.T) {
		t.Parallel()
		rec := mx.Record{Priority: 10, Host: "mail.example.com.", TTL: 300}
		assert.Equal(t, uint32(300), rec.RecordTTL())
	})
}

func TestRecord_Empty(t *testing.T) {
	t.Parallel()
	assert.True(t, (&mx.Record{}).Empty())
	assert.False(t, (&mx.Record{Priority: 10, Host: "mail.example.com."}).Empty())
}

func TestRecord_Render_defaultHost(t *testing.T) {
	t.Parallel()

	rec := mx.Record{Priority: 10}
	type data struct{ Domain string }
	err := rec.Render(&data{Domain: "dmarc-reports.example.dioad.dev"})
	require.NoError(t, err)
	assert.Equal(t, "dmarc-reports.example.dioad.dev.", rec.Host)
}

func TestRecord_Render_explicitHost(t *testing.T) {
	t.Parallel()

	rec := mx.Record{Priority: 10, Host: "smtp.{{.Domain}}."}
	type data struct{ Domain string }
	err := rec.Render(&data{Domain: "example.dioad.dev"})
	require.NoError(t, err)
	assert.Equal(t, "smtp.example.dioad.dev.", rec.Host)
}

func TestRecord_Render_invalidTemplate(t *testing.T) {
	t.Parallel()
	rec := mx.Record{Priority: 10, Host: "{{.Unclosed"}
	err := rec.Render(struct{}{})
	assert.Error(t, err)
}

func TestRecord_RecordValue_afterRender(t *testing.T) {
	t.Parallel()

	rec := mx.Record{Priority: 10}
	type data struct{ Domain string }
	require.NoError(t, rec.Render(&data{Domain: "dmarc-reports.example.dioad.dev"}))
	assert.Equal(t, "10 dmarc-reports.example.dioad.dev.", rec.RecordValue())
}
