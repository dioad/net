package dns

import (
	"fmt"
	"time"

	"github.com/miekg/dns"

	nettls "github.com/dioad/net/tls"
	"github.com/dioad/util"
)

// TLSARecord is a TLSA/DANE record whose value is computed from the X.509
// certificate at a templated file path. Content is fetched lazily on first
// Render and, if AutoRefresh is set, kept fresh afterward by a background
// ticker running every AutoRefreshPeriodSeconds -- see refreshableContent.
type TLSARecord struct {
	// CertPathTemplate is a Go template string (expanded against the data
	// passed to Render) for the path to the PEM-encoded certificate file
	// this record's value is derived from.
	CertPathTemplate string `mapstructure:"cert-path-template"`

	// Name is the DNS owner label this record targets, e.g. "mx" for a TLSA
	// record on a mail exchanger.
	Name string `mapstructure:"name"`

	// Port and Proto identify the service this TLSA record applies to,
	// e.g. Port 25, Proto "tcp" for SMTP.
	Port  uint8  `mapstructure:"port"`
	Proto string `mapstructure:"proto"`

	// Selector, MatchingType, and Usage are the TLSA record's own fields,
	// as defined in RFC 6698.
	Selector     uint8 `mapstructure:"selector"`
	MatchingType uint8 `mapstructure:"matching-type"`
	Usage        uint8 `mapstructure:"usage"`

	AutoRefresh              bool  `mapstructure:"auto-refresh"`
	AutoRefreshPeriodSeconds int64 `mapstructure:"auto-refresh-period-seconds"`

	content refreshableContent
}

// fetchDNSContents derives the TLSA record value from the certificate at
// CertPathTemplate (expanded against data).
func (r *TLSARecord) fetchDNSContents(data any) (string, error) {
	certPath, err := util.ExpandStringTemplate(r.CertPathTemplate, data)
	if err != nil {
		return "", err
	}

	cert, err := nettls.LoadX509CertFromFile(certPath)
	if err != nil {
		return "", err
	}

	tlsaValue, err := dns.CertificateToDANE(r.Selector, r.MatchingType, cert)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%d %d %d %v", r.Selector, r.MatchingType, r.Usage, tlsaValue), nil
}

func (r *TLSARecord) Render(data any) error {
	period := time.Duration(r.AutoRefreshPeriodSeconds) * time.Second
	return r.content.Render(func() (string, error) { return r.fetchDNSContents(data) }, r.AutoRefresh, period)
}

// Empty reports whether this record has no configuration and has not yet
// fetched any content.
func (r *TLSARecord) Empty() bool {
	return r.CertPathTemplate == "" && r.content.Value() == ""
}

func (r *TLSARecord) RecordPrefix() string {
	return fmt.Sprintf("_%d._%s.%s.", r.Port, r.Proto, r.Name)
}

func (r *TLSARecord) RecordType() string { return "TLSA" }

func (r *TLSARecord) RecordValue() string { return r.content.Value() }

func (r *TLSARecord) String() string { return r.content.Value() }

var (
	_ DNSRecord       = (*TLSARecord)(nil)
	_ TemplatedRecord = (*TLSARecord)(nil)
)
