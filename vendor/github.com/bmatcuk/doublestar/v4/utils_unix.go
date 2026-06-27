//go:build !windows

package doublestar

import (
	"io/fs"
)

// Returns true if the pattern could "unintentionally" match hidden files/dirs.
// An unintentional pattern would use a meta character that could match
// anything.
func couldUnintentionallyMatchHidden(pattern string) bool {
	return len(pattern) > 0 && (pattern[0] == '*' || pattern[0] == '?')
}

// Returns true if the file is "hidden"
func isHiddenPath(filename string, _info fs.DirEntry) (bool, error) {
	return filename[0] == '.', nil
}
