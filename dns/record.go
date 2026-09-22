package dns

import (
	"fmt"

	"github.com/dioad/util"
)

// DefaultTTL is the DNS TTL used when a record does not specify one.
const DefaultTTL uint32 = 60

// DNSRecord is the canonical interface for DNS record types across this module.
// Existing smtp sub-package records implement it by structural compatibility;
// the coredns package imports this definition rather than defining its own.
//
//nolint:revive // stutters (dns.DNSRecord), but it's a public interface external packages already import by this name; renaming it is a breaking change of its own, out of scope here
type DNSRecord interface {
	// RecordPrefix returns the DNS owner label prefix relative to the base domain,
	// e.g. "_dmarc." or "" for the apex.
	RecordPrefix() string
	// RecordType returns the DNS record type string, e.g. "TXT" or "MX".
	RecordType() string
	// RecordValue returns the wire-ready record value for insertion into a
	// CoreDNS template answer line.
	RecordValue() string
}

// TemplatedRecord is satisfied by records whose field values contain Go template
// strings that must be expanded before the record is served.
// It matches the interface already defined in github.com/dioad/net/smtp.
type TemplatedRecord interface {
	Render(data any) error
	Empty() bool
}

// TXTRecord is a generic DNS TXT record with optional Go template expansion in Value.
type TXTRecord struct {
	// Name is the DNS owner label relative to the base domain. "" = apex.
	Name string `json:"name,omitempty" mapstructure:"name"`
	// Value is a Go template string for the TXT record content (e.g. "v=spf1 {{.OutboundIPList}} -all").
	Value string `json:"value" mapstructure:"value"`
	// TTL is the DNS TTL in seconds advertised to resolvers. Zero uses DefaultTTL.
	TTL uint32 `json:"ttl,omitempty" mapstructure:"ttl"`
}

// RecordPrefix returns the DNS owner label prefix relative to the base
// domain, e.g. "sub." or "" for the apex.
func (r *TXTRecord) RecordPrefix() string {
	if r.Name == "" {
		return ""
	}

	return r.Name + "."
}

// RecordType returns the DNS record type string, "TXT".
func (r *TXTRecord) RecordType() string { return "TXT" }

// RecordValue returns the wire-ready, quoted TXT record value.
func (r *TXTRecord) RecordValue() string { return fmt.Sprintf(`\"%s\"`, r.Value) }

// Empty reports whether Value is unset.
func (r *TXTRecord) Empty() bool { return r.Value == "" }

// RecordTTL returns the TTL to use for this record, falling back to DefaultTTL.
func (r *TXTRecord) RecordTTL() uint32 {
	if r.TTL == 0 {
		return DefaultTTL
	}

	return r.TTL
}

// Render expands Go template directives in Value (e.g. {{.Domain}}, {{.Email}}).
func (r *TXTRecord) Render(data any) error {
	rendered, err := util.ExpandStringTemplate(r.Value, data)
	if err != nil {
		return fmt.Errorf("rendering TXT value: %w", err)
	}
	r.Value = rendered

	return nil
}

var (
	_ DNSRecord       = (*TXTRecord)(nil)
	_ TemplatedRecord = (*TXTRecord)(nil)
)
