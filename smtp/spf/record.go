package spf

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/dioad/net"
)

const (
	QualifierNone     Qualifier = ""
	QualifierPass     Qualifier = "+"
	QualifierSoftFail Qualifier = "~"
	QualifierFail     Qualifier = "-"
	QualifierNeutral  Qualifier = "?"
)

type Qualifier string

type Mechanism struct {
	Qualifier  Qualifier           `mapstructure:"qualifier"`
	Name       string              `mapstructure:"name"`
	Values     []string            `mapstructure:"values"`
	ValueList  string              `mapstructure:"value-list"`
	ValuesFunc MechanismValuesFunc `mapstructure:"-" json:"-"`
}

func resolveValues(mech Mechanism) []string {
	values := make([]string, 0)
	if len(mech.Values) > 0 {
		values = append(values, mech.Values...)
	}

	if mech.ValueList != "" {
		values = append(values, strings.Split(mech.ValueList, ",")...)
	}

	if mech.ValuesFunc != nil {
		values = append(values, mech.ValuesFunc()...)
	}
	return values
}

func IP4Mechanism(values ...string) Mechanism {
	return Mechanism{Name: "ip4", Values: values}
}

func IP6Mechanism(values ...string) Mechanism {
	return Mechanism{Name: "ip6", Values: values}
}

func MXMechanism(values ...string) Mechanism {
	return Mechanism{Name: "mx", Values: values}
}

func AMechanism(values ...string) Mechanism {
	return Mechanism{Name: "a", Values: values}
}

func IncludeMechanism(values ...string) Mechanism {
	return Mechanism{Name: "include", Values: values}
}

type MechanismValuesFunc func() []string

type Record struct {
	Version    string      `mapstructure:"version"`
	Mechanisms []Mechanism `mapstructure:"mechanisms"`

	All          bool      `mapstructure:"all"`
	AllQualifier Qualifier `mapstructure:"all-qualifier"`
}

func (r *Record) Empty() bool {
	return reflect.DeepEqual(*r, Record{})
}

func (r *Record) Add(m Mechanism) {
	r.Mechanisms = append(r.Mechanisms, m)
}

func (r *Record) Render(data interface{}) error {
	for i := range r.Mechanisms {
		// There's gotta be a cleaner way to do this
		renderedValueList, err := net.ExpandStringTemplate(r.Mechanisms[i].ValueList, data)
		if err != nil {
			return err
		}
		r.Mechanisms[i].ValueList = renderedValueList

		values := resolveValues(r.Mechanisms[i])
		for j := range values {
			renderedValue, err := net.ExpandStringTemplate(values[j], data)
			if err != nil {
				return err
			}
			values[j] = renderedValue
		}
		r.Mechanisms[i].Values = values
	}
	return nil
}

func (r *Record) RecordPrefix() string {
	return ""
}

func (r *Record) RecordType() string {
	return "TXT"
}

func (r *Record) RecordValue() string {
	return fmt.Sprintf("\\\"%v\\\"", r.String())
}

func (r *Record) String() string {
	parts := make([]string, 0)

	if r.Version == "" {
		parts = append(parts, "v=spf1")
	} else {
		parts = append(parts, fmt.Sprintf("v=%s", r.Version))
	}

	for _, m := range r.Mechanisms {
		parts = append(parts, FormatMechanism(m))
	}

	if r.All {
		parts = append(parts, fmt.Sprintf("%sall", r.AllQualifier))
	}

	result := strings.Join(parts, " ")
	if len(result) > 255 {
		panic(fmt.Sprintf("too many chars for Record record: %s", result))
	}
	return result
}

func FormatMechanism(mechanism Mechanism) string {
	if len(mechanism.Values) == 0 {
		return ""
	}
	outputs := make([]string, 0, len(mechanism.Values))
	for _, m := range mechanism.Values {
		outputs = append(outputs, fmt.Sprintf("%s:%s", mechanism.Name, m))
	}

	return strings.Join(outputs, " ")
}
