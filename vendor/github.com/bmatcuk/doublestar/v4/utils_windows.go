package doublestar

import (
	"io/fs"
	"syscall"
)

// Returns true if the pattern could "unintentionally" match hidden files/dirs.
// An unintentional pattern would use a meta character that could match
// anything.
func couldUnintentionallyMatchHidden(pattern string) bool {
	var c byte
	inClass := false
	l := len(pattern)
	for i := 0; i < l; i++ {
		c = pattern[i]
		if !inClass && (c == '*' || c == '?') {
			return true
		} else if c == '[' {
			inClass = true
		} else if c == '\\' {
			// skip next byte
			i++
		} else if inClass && c == ']' {
			inClass = false
		}
	}
	return false
}

// Returns true if the file is "hidden"
func isHiddenPath(_filename string, info fs.DirEntry) (bool, error) {
	fileinfo, err := info.Info()
	if err != nil {
		return false, err
	}

	if stat, ok := fileinfo.Sys().(*syscall.Win32FileAttributeData); ok {
		return stat.FileAttributes&syscall.FILE_ATTRIBUTE_HIDDEN != 0, nil
	}

	return false, nil
}
