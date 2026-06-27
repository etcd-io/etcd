package pqutil

import (
	"strconv"
	"strings"
)

// ParseBool is like strconv.ParseBool, but also accepts "yes"/"no" and
// "on"/"off".
func ParseBool(str string) (bool, error) {
	switch str {
	case "1", "t", "T", "true", "TRUE", "True", "yes", "on":
		return true, nil
	case "0", "f", "F", "false", "FALSE", "False", "no", "off":
		return false, nil
	}
	return false, &strconv.NumError{Func: "ParseBool", Num: str, Err: strconv.ErrSyntax}
}

func Join[S ~[]E, E ~string](s S) string {
	var b strings.Builder
	for i := range s {
		if i > 0 {
			b.WriteString(", ")
		}
		if i == len(s)-1 {
			b.WriteString("or ")
		}
		b.WriteString(string(s[i]))
	}
	return b.String()
}
