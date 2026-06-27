//go:build windows

package fsutils

import (
	"path/filepath"
	"regexp"
	"strings"
)

var separatorToReplace = regexp.QuoteMeta(string(filepath.Separator))

// NormalizePathInRegex normalizes path in regular expressions.
// noop on Unix.
// This replacing should be safe because "/" are disallowed in Windows
// https://docs.microsoft.com/windows/win32/fileio/naming-a-file
func NormalizePathInRegex(path string) string {
	// remove redundant character escape "\/" https://github.com/golangci/golangci-lint/issues/3277
	clean := regexp.MustCompile(`\\+/`).
		ReplaceAllStringFunc(path, func(s string) string {
			if strings.Count(s, "\\")%2 == 0 {
				return s
			}
			return s[1:]
		})

	return strings.ReplaceAll(clean, "/", separatorToReplace)
}
