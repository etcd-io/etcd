package string

import (
	"strings"
)

// TrimSplit slices s into all substrings separated by sep and returns a
// slice of the substrings between the separator with all leading and trailing
// white space removed, as defined by Unicode.
func TrimSplit(s, sep string) []string {
	trimmed := strings.Split(s, sep)
	for i := range trimmed {
		trimmed[i] = strings.TrimSpace(trimmed[i])
	}
	return trimmed
}

// Clone returns a copy of the string, so that we can safely point to the
// copy without worrying about changes via pointers.
func Clone(s string) string {
	stringCopy := s
	return stringCopy
}
