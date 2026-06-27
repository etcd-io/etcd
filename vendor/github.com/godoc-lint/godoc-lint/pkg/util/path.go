package util

import (
	"path/filepath"
	"strings"
)

// IsPathUnderBaseDir determines whether the given path is a sub-directory of
// the given base, lexicographically.
func IsPathUnderBaseDir(baseDir, path string) bool {
	rel, err := filepath.Rel(baseDir, path)
	if err != nil {
		return false
	}
	return rel == "." || rel != ".." && !strings.HasPrefix(filepath.ToSlash(rel), "../")
}
