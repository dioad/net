package dmarc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dioad/net/dns"
)

func TestRecordRendersItsReportAddressesThroughTemplatedRecord(t *testing.T) {
	t.Parallel()

	var r dns.DNSRecord = &Record{Policy: PolicyQuarantine, ReportURIAggregate: []string{"dmarc@{{ .Domain }}"}}
	templated, ok := r.(dns.TemplatedRecord)
	require.True(t, ok, "a DNS server only expands the templates of records that satisfy dns.TemplatedRecord")

	require.NoError(t, templated.Render(struct{ Domain string }{"example.com"}))

	assert.Contains(t, r.RecordValue(), "rua=mailto:dmarc@example.com")
}

func TestRecordEmpty(t *testing.T) {
	t.Parallel()

	assert.True(t, (&Record{}).Empty())
	assert.False(t, (&Record{Policy: PolicyReject}).Empty())
}
