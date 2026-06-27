package utils

import (
	"strings"
)

type MultiError []error

func (me MultiError) Error() string {
	b := strings.Builder{}
	for i, e := range me {
		b.WriteString(e.Error())
		if i < len(me)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
