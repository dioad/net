package spf

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dioad/net/dns"
)

// renderAsTemplated renders r the way a DNS server does: through
// dns.TemplatedRecord, which it only calls on records that satisfy it.
func renderAsTemplated(t *testing.T, r dns.DNSRecord, data any) {
	t.Helper()
	templated, ok := r.(dns.TemplatedRecord)
	require.True(t, ok, "%T must satisfy dns.TemplatedRecord or its templates are never expanded", r)
	require.NoError(t, templated.Render(data))
}

func TestRecordRendersItsValueListThroughTemplatedRecord(t *testing.T) {
	t.Parallel()

	r := &Record{
		Mechanisms:   []Mechanism{{Name: "ip", ValueList: "{{ .OutboundIPList }}"}},
		All:          true,
		AllQualifier: QualifierFail,
	}

	renderAsTemplated(t, r, struct{ OutboundIPList string }{"192.0.2.1,2001:db8::1"})

	assert.Equal(t, "v=spf1 ip4:192.0.2.1 ip6:2001:db8::1 -all", r.String())
}

func TestRenderingCopiesOfARecordDoesNotRepeatValues(t *testing.T) {
	t.Parallel()

	source := Record{
		Mechanisms:   []Mechanism{{Name: "ip", ValueList: "{{ .OutboundIPList }}"}},
		All:          true,
		AllQualifier: QualifierFail,
	}
	data := struct{ OutboundIPList string }{"192.0.2.1"}

	first := source
	require.NoError(t, first.Render(data))
	second := source
	require.NoError(t, second.Render(data))

	assert.Equal(t, "v=spf1 ip4:192.0.2.1 -all", second.String(), "rendering a copy repeats no value")
	assert.Empty(t, source.Mechanisms[0].Values, "rendering a copy leaves the source's mechanisms unrendered")
}

func TestRecordEmpty(t *testing.T) {
	t.Parallel()

	assert.True(t, (&Record{}).Empty())
	assert.False(t, (&Record{All: true}).Empty())
	assert.False(t, (&Record{Mechanisms: []Mechanism{AMechanism("mx.example.com")}}).Empty())
}
