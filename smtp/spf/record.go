package spf

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"text/template"
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
	Qualifier Qualifier `mapstructure:"qualifier"`
	Name      string    `mapstructure:"name"`
	Values    []string  `mapstructure:"values"`
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
		for j := range r.Mechanisms[i].Values {
			tmpl, err := template.New("spf").Parse(r.Mechanisms[i].Values[j])
			if err != nil {
				return err
			}
			buf := &bytes.Buffer{}
			tmpl.Execute(buf, data)
			r.Mechanisms[i].Values[j] = buf.String()
		}
	}
	return nil
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
