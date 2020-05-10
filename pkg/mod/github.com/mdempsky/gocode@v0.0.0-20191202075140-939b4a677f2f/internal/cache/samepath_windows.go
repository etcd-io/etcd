// +build windows

package cache

import (
	"strings"
)

// SamePath checks two file paths for their equality based on the current filesystem
func SamePath(a, b string) bool {
	return strings.EqualFold(a, b)
}
