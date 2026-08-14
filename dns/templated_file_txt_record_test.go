package dns_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dioad/net/dns"
)

type fileTXTRenderData struct {
	Domain string
}

func writeDKIMFixture(t *testing.T, stateDir, domain, content string) {
	t.Helper()

	dir := filepath.Join(stateDir, "dkim_keys")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	path := filepath.Join(dir, domain+"_default.dns")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func newTemplatedFileTXTRecord(stateDir string) *dns.TemplatedFileTXTRecord {
	return &dns.TemplatedFileTXTRecord{
		PathTemplate: filepath.Join(stateDir, "dkim_keys", "{{.Domain}}_default.dns"),
		Name:         "default._domainkey",
	}
}

func TestTemplatedFileTXTRecord_RecordPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rec      dns.TemplatedFileTXTRecord
		expected string
	}{
		{"apex (empty name)", dns.TemplatedFileTXTRecord{}, ""},
		{"selector label", dns.TemplatedFileTXTRecord{Name: "default._domainkey"}, "default._domainkey."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, tc.rec.RecordPrefix())
		})
	}
}

func TestTemplatedFileTXTRecord_RecordType(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "TXT", (&dns.TemplatedFileTXTRecord{}).RecordType())
}

func TestTemplatedFileTXTRecord_Render_FetchesOnce(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	writeDKIMFixture(t, stateDir, "example.com", "v=DKIM1; k=rsa; p=first")

	r := newTemplatedFileTXTRecord(stateDir)
	data := fileTXTRenderData{Domain: "example.com"}

	require.NoError(t, r.Render(data))
	first := r.RecordValue()
	assert.Contains(t, first, "first")

	// Overwrite the fixture; Render's fetch-once guard means a second call
	// must not re-read it.
	writeDKIMFixture(t, stateDir, "example.com", "v=DKIM1; k=rsa; p=second")
	require.NoError(t, r.Render(data))

	assert.Equal(t, first, r.RecordValue())
}

// TestTemplatedFileTXTRecord_Render_AutoRefresh_NoRace exercises the
// AutoRefresh path under -race: fetchDNSContents writes the cached value
// from the ticker goroutine while RecordValue/String read it concurrently
// from the test goroutine.
func TestTemplatedFileTXTRecord_Render_AutoRefresh_NoRace(t *testing.T) {
	stateDir := t.TempDir()
	writeDKIMFixture(t, stateDir, "example.com", "v=DKIM1; k=rsa; p=initial")

	r := newTemplatedFileTXTRecord(stateDir)
	r.AutoRefresh = true
	r.AutoRefreshPeriodSeconds = 1
	data := fileTXTRenderData{Domain: "example.com"}

	require.NoError(t, r.Render(data))

	deadline := time.After(1500 * time.Millisecond)
	for {
		select {
		case <-deadline:
			assert.NotEmpty(t, r.RecordValue())
			assert.NotEmpty(t, r.String())
			return
		default:
			_ = r.RecordValue()
			_ = r.String()
		}
	}
}
