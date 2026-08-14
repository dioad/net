package dns

import (
	"fmt"
	"io"
	"time"

	"github.com/dioad/util"
)

// TemplatedFileTXTRecord is a DNS TXT record whose value is the contents of
// a file at a templated path. Content is fetched lazily on first Render
// and, if AutoRefresh is set, kept fresh afterward by a background ticker
// running every AutoRefreshPeriodSeconds -- see refreshableContent.
type TemplatedFileTXTRecord struct {
	// PathTemplate is a Go template string (expanded against the data passed
	// to Render) for the path to the file this record's value is read from.
	PathTemplate string `mapstructure:"path-template"`

	// Name is the DNS owner label this record targets, e.g.
	// "default._domainkey" for a DKIM selector record. "" targets the apex.
	Name string `mapstructure:"name"`

	AutoRefresh              bool  `mapstructure:"auto-refresh"`
	AutoRefreshPeriodSeconds int64 `mapstructure:"auto-refresh-period-seconds"`

	content refreshableContent
}

// fetchDNSContents reads the file at PathTemplate (expanded against data).
func (r *TemplatedFileTXTRecord) fetchDNSContents(data any) (string, error) {
	path, err := util.ExpandStringTemplate(r.PathTemplate, data)
	if err != nil {
		return "", err
	}

	f, err := util.CleanOpen(path)
	if err != nil {
		return "", err
	}

	contents, err := io.ReadAll(f)
	if err != nil {
		_ = f.Close()
		return "", err
	}

	if err := f.Close(); err != nil {
		return "", err
	}

	return string(contents), nil
}

func (r *TemplatedFileTXTRecord) Render(data any) error {
	period := time.Duration(r.AutoRefreshPeriodSeconds) * time.Second
	return r.content.Render(func() (string, error) { return r.fetchDNSContents(data) }, r.AutoRefresh, period)
}

// Empty reports whether this record has no configuration and has not yet
// fetched any content.
func (r *TemplatedFileTXTRecord) Empty() bool {
	return r.PathTemplate == "" && r.content.Value() == ""
}

func (r *TemplatedFileTXTRecord) RecordPrefix() string {
	if r.Name == "" {
		return ""
	}
	return r.Name + "."
}

func (r *TemplatedFileTXTRecord) RecordType() string { return "TXT" }

func (r *TemplatedFileTXTRecord) RecordValue() string {
	return fmt.Sprintf("\\\"%v\\\"", r.content.Value())
}

func (r *TemplatedFileTXTRecord) String() string { return r.content.Value() }

var (
	_ DNSRecord       = (*TemplatedFileTXTRecord)(nil)
	_ TemplatedRecord = (*TemplatedFileTXTRecord)(nil)
)
