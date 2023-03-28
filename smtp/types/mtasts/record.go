package mtasts

import (
	"fmt"
	"strings"
)

type Record struct {
	Version string
	ID      []string
}

func (r *Record) String() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("v=%s ", r.Version))
	sb.WriteString(fmt.Sprintf("id=%s", r.ID))
	
	result := sb.String()
	if len(result) > 255 {
		panic(fmt.Sprintf("too many chars for Record record: %s", result))
	}
	return result
}
