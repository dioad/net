// Package mx provides a DNS MX record type for use with connect tunnels.
package mx

import (
	"fmt"

	"github.com/dioad/net/dns"
	"github.com/dioad/util"
)

// Record is a DNS MX record following the same Render pattern as dmarc.Record
// and spf.Record: fields may contain Go template strings expanded at render time.
type Record struct {
	// Priority is the MX preference value.
	Priority uint16 `mapstructure:"priority" json:"priority"`
	// Host is a Go template string for the mail exchange FQDN (must end in ".").
	// Empty defaults to "{{.Domain}}." at render time — the tunnel's own FQDN.
	Host string `mapstructure:"host" json:"host,omitempty"`
	// Prefix is the DNS owner label relative to the base domain. "" = apex.
	Prefix string `mapstructure:"prefix" json:"prefix,omitempty"`
	// TTL is the DNS TTL in seconds advertised to resolvers. Zero uses dns.DefaultTTL.
	TTL uint32 `mapstructure:"ttl" json:"ttl,omitempty"`
}

func (r *Record) RecordPrefix() string {
	if r.Prefix == "" {
		return ""
	}
	return r.Prefix + "."
}

func (r *Record) RecordType() string { return "MX" }

func (r *Record) RecordValue() string { return fmt.Sprintf("%d %s", r.Priority, r.Host) }

func (r *Record) Empty() bool { return r.Priority == 0 && r.Host == "" }

// RecordTTL returns the TTL to use for this record, falling back to dns.DefaultTTL.
func (r *Record) RecordTTL() uint32 {
	if r.TTL == 0 {
		return dns.DefaultTTL
	}
	return r.TTL
}

// Render expands the Host template. An empty Host defaults to "{{.Domain}}."
// before expansion so the MX target resolves to the tunnel's own FQDN.
func (r *Record) Render(data any) error {
	host := r.Host
	if host == "" {
		host = "{{.Domain}}."
	}
	rendered, err := util.ExpandStringTemplate(host, data)
	if err != nil {
		return fmt.Errorf("rendering MX host: %w", err)
	}
	r.Host = rendered
	return nil
}

var (
	_ dns.DNSRecord       = (*Record)(nil)
	_ dns.TemplatedRecord = (*Record)(nil)
)
