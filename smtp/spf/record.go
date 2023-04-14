package spf

import (
	"fmt"
	"strings"
)

const (
	QualifierNone     Qualifier = ""
	QualifierPass               = "+"
	QualifierSoftFail           = "~"
	QualifierFail               = "-"
	QualifierNeutral            = "?"
)

type Qualifier string

type Mechanism struct {
	Qualifier Qualifier
	Name      string
	Values    []string
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
	Version    string
	Mechanisms []Mechanism

	All          bool
	AllQualifier Qualifier
}

func (r *Record) Add(m Mechanism) {
	r.Mechanisms = append(r.Mechanisms, m)
}

func (r *Record) String() string {
	parts := make([]string, 0)
	parts = append(parts, fmt.Sprintf("v=%s", r.Version))

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
