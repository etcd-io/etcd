package stringutil

import "unicode/utf8"

// IsASCII returns true if string are ASCII.
func IsASCII(s string) bool {
	for _, r := range s {
		if r >= utf8.RuneSelf {
			// Not ASCII.
			return false
		}
	}

	return true
}
