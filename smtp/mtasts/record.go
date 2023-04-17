package mtasts

import (
	"fmt"
	"strings"
)

type Record struct {
	Version string `mapstructure:"version"`
	ID      string `mapstructure:"id"`
}

func (r *Record) String() string {
	var sb strings.Builder

	if r.Version == "" {
		sb.WriteString("v=STSv1; ")
	} else {
		sb.WriteString(fmt.Sprintf("v=%s ", r.Version))
	}
	sb.WriteString(fmt.Sprintf("id=%s", r.ID))

	result := sb.String()
	if len(result) > 255 {
		panic(fmt.Sprintf("too many chars for Record record: %s", result))
	}
	return result
}
