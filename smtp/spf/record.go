// Package spf provides types for building an SPF DNS TXT record.
package spf

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/dioad/filter"
	"github.com/dioad/generics"
	"github.com/dioad/util"

	"github.com/dioad/net/smtp/internal/txtchunk"
)

// SPF mechanism qualifiers, per RFC 7208 section 4.6.1.
const (
	QualifierNone     Qualifier = ""
	QualifierPass     Qualifier = "+"
	QualifierSoftFail Qualifier = "~"
	QualifierFail     Qualifier = "-"
	QualifierNeutral  Qualifier = "?"
)

// Qualifier represents an SPF mechanism qualifier (e.g., "+", "-", "~", "?").
type Qualifier string

// Mechanism represents an SPF mechanism in a DNS record.
type Mechanism struct {
	Qualifier  Qualifier           `mapstructure:"qualifier"`
	Name       string              `mapstructure:"name"`
	Values     []string            `mapstructure:"values"`
	ValueList  string              `mapstructure:"value-list"`
	ValuesFunc MechanismValuesFunc `json:"-"                  mapstructure:"-"`
}

// IPMechanisms splits values (a mix of IPv4 and IPv6 addresses) into a
// separate "ip4" mechanism and "ip6" mechanism, either of which is nil if
// no addresses of that family were present.
func IPMechanisms(values ...string) (*Mechanism, *Mechanism) {
	var ip4Mechanism *Mechanism
	var ip6Mechanism *Mechanism

	for _, v := range values {
		ip, _ := netip.ParseAddr(v)
		if ip.Is6() {
			if ip6Mechanism == nil {
				ip6Mechanism = &Mechanism{Name: "ip6", Values: []string{}}
			}
			ip6Mechanism.Values = append(ip6Mechanism.Values, v)
		} else {
			if ip4Mechanism == nil {
				ip4Mechanism = &Mechanism{Name: "ip4", Values: []string{}}
			}
			ip4Mechanism.Values = append(ip4Mechanism.Values, v)
		}
	}

	return ip4Mechanism, ip6Mechanism
}

// IP4Mechanism returns an "ip4" mechanism listing values.
func IP4Mechanism(values ...string) Mechanism {
	return Mechanism{Name: "ip4", Values: values}
}

// IP6Mechanism returns an "ip6" mechanism listing values.
func IP6Mechanism(values ...string) Mechanism {
	return Mechanism{Name: "ip6", Values: values}
}

// MXMechanism returns an "mx" mechanism listing values.
func MXMechanism(values ...string) Mechanism {
	return Mechanism{Name: "mx", Values: values}
}

// AMechanism returns an "a" mechanism listing values.
func AMechanism(values ...string) Mechanism {
	return Mechanism{Name: "a", Values: values}
}

// IncludeMechanism returns an "include" mechanism listing values.
func IncludeMechanism(values ...string) Mechanism {
	return Mechanism{Name: "include", Values: values}
}

// MechanismValuesFunc is a function type that returns mechanism values.
type MechanismValuesFunc func() []string

// Record represents an SPF DNS record.
type Record struct {
	Version    string      `mapstructure:"version"`
	Mechanisms []Mechanism `mapstructure:"mechanisms"`

	All          bool      `mapstructure:"all"`
	AllQualifier Qualifier `mapstructure:"all-qualifier"`
}

// Add appends m to the record's mechanisms.
func (r *Record) Add(m Mechanism) {
	r.Mechanisms = append(r.Mechanisms, m)
}

func resolveValues(mech Mechanism, data any) []string {
	values := make([]string, 0)
	if len(mech.Values) > 0 {
		expandedValues := generics.SafeMap(func(s string) string {
			expanded, _ := util.ExpandStringTemplate(s, data)

			return expanded
		}, mech.Values)
		values = append(values, expandedValues...)
	}

	if mech.ValueList != "" {
		expandedValueList, _ := util.ExpandStringTemplate(mech.ValueList, data)
		values = append(values, strings.Split(expandedValueList, ",")...)
	}

	if mech.ValuesFunc != nil {
		values = append(values, mech.ValuesFunc()...)
	}

	return values
}

// Render expands each mechanism's ValueList/Values templates against data.
func (r *Record) Render(data any) error {
	for i := range r.Mechanisms {
		r.Mechanisms[i].Values = resolveValues(r.Mechanisms[i], data)
	}

	return nil
}

// RecordPrefix returns the DNS owner label prefix for an SPF record: always
// "" (the apex).
func (r *Record) RecordPrefix() string {
	return ""
}

// RecordType returns the DNS record type string, "TXT".
func (r *Record) RecordType() string {
	return "TXT"
}

// RecordValue returns the wire-ready, quoted SPF record value.
func (r *Record) RecordValue() string {
	return txtchunk.Quote(r.String())
}

func (r *Record) String() string {
	parts := make([]string, 0)

	if r.Version == "" {
		parts = append(parts, "v=spf1")
	} else {
		parts = append(parts, "v="+r.Version)
	}

	ipMechanisms := filter.FilterSlice(r.Mechanisms, func(m Mechanism) bool { return m.Name == "ip" })
	nonIPMechanisms := filter.FilterSlice(r.Mechanisms, func(m Mechanism) bool { return m.Name != "ip" })

	for _, ipMechanism := range ipMechanisms {
		if formatted := formatIPMechanism(ipMechanism); formatted != "" {
			parts = append(parts, formatted)
		}
	}

	mechanisms := FormatMechanisms(nonIPMechanisms...)
	if mechanisms != "" {
		parts = append(parts, mechanisms)
	}

	if r.All {
		parts = append(parts, fmt.Sprintf("%sall", r.AllQualifier))
	}

	return strings.Join(parts, " ")
}

// formatIPMechanism formats the ip4/ip6 mechanisms parsed from a single
// combined "ip" mechanism's values, joining both onto one entry when both
// are present. Returns "" if neither is present.
func formatIPMechanism(ipMechanism Mechanism) string {
	ip4Mechanism, ip6Mechanism := IPMechanisms(ipMechanism.Values...)

	switch {
	case ip4Mechanism != nil && ip6Mechanism != nil:
		return FormatMechanisms(*ip4Mechanism, *ip6Mechanism)
	case ip4Mechanism != nil:
		return FormatMechanisms(*ip4Mechanism)
	case ip6Mechanism != nil:
		return FormatMechanisms(*ip6Mechanism)
	default:
		return ""
	}
}

// FormatMechanism formats a single mechanism as "<qualifier><name>:<value>"
// entries (space-separated when it has multiple values), e.g. "ip4:1.2.3.4".
func FormatMechanism(mechanism Mechanism) string {
	if len(mechanism.Values) == 0 {
		return ""
	}
	outputs := make([]string, 0, len(mechanism.Values))
	for _, m := range mechanism.Values {
		outputs = append(outputs, fmt.Sprintf("%s%s:%s", mechanism.Qualifier, mechanism.Name, m))
	}

	return strings.Join(outputs, " ")
}

// FormatMechanisms formats each mechanism (see FormatMechanism) and joins
// the non-empty results with a space.
func FormatMechanisms(mechanism ...Mechanism) string {
	outputs := make([]string, 0, len(mechanism))
	for _, m := range mechanism {
		outputs = append(outputs, FormatMechanism(m))
	}

	return strings.Join(outputs, " ")
}
