package smtp

import (
	"github.com/dioad/net/smtp/dkim"
	"github.com/dioad/net/smtp/dmarc"
	"github.com/dioad/net/smtp/mtasts"
	"github.com/dioad/net/smtp/spf"
	"github.com/dioad/net/smtp/tlsrpt"
)

// DomainRecords holds all the DNS records required for domain authentication.
type DomainRecords struct {
	TLSRPT tlsrpt.Record `mapstructure:"tls-rpt"`
	DKIM   dkim.Record   `mapstructure:"dkim"`
	DMARC  dmarc.Record  `mapstructure:"dmarc"`
	MTASTS mtasts.Record `mapstructure:"mta-sts"`
	SPF    spf.Record    `mapstructure:"spf"`
}

func (r *DomainRecords) Render(data interface{}) error {
	if err := r.DMARC.Render(data); err != nil {
		return err
	}

	if err := r.SPF.Render(data); err != nil {
		return err
	}

	return nil
}

// TemplatedRecord defines an interface for domain records that support templating.
type TemplatedRecord interface {
	Render(data interface{}) error
	Empty() bool
}
