package lint

import (
	"fmt"
	"regexp"
	"strings"
)

// FileFilter filters file to exclude some files for a rule.
// Supports the following:
//   - File or directory names: pkg/mypkg/my.go
//   - Globs: **/*.pb.go,
//   - Regexes (with ~ prefix): ~-tmp\.\d+\.go
//   - Special test marker `TEST` (treated as `~_test\.go`).
type FileFilter struct {
	// raw definition of filter inside config
	raw string
	// don't care what was at start, will use regexes inside
	rx *regexp.Regexp
	// marks filter as matching everything
	matchesAll bool
	// marks filter as matching nothing
	matchesNothing bool
}

// ParseFileFilter creates a [FileFilter] for the given raw filter.
// If the string is empty, it matches nothing.
// If the string is `*` or `~`, it matches everything.
// If the regular expression is invalid, it returns a compilation error.
func ParseFileFilter(rawFilter string) (*FileFilter, error) {
	rawFilter = strings.TrimSpace(rawFilter)
	result := new(FileFilter)
	result.raw = rawFilter
	result.matchesNothing = result.raw == ""
	result.matchesAll = result.raw == "*" || result.raw == "~"
	if !result.matchesAll && !result.matchesNothing {
		if err := result.prepareRegexp(); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// String returns the original raw filter definition as it appears in the configuration.
func (ff *FileFilter) String() string { return ff.raw }

// MatchFileName checks if the file name matches the filter.
func (ff *FileFilter) MatchFileName(name string) bool {
	if ff.matchesAll {
		return true
	}
	if ff.matchesNothing {
		return false
	}
	name = strings.ReplaceAll(name, "\\", "/")
	return ff.rx.MatchString(name)
}

var (
	fileFilterInvalidGlobRegexp = regexp.MustCompile(`[^/]\*\*[^/]`)
	escapeRegexSymbols          = ".+{}()[]^$"
)

func (ff *FileFilter) prepareRegexp() error {
	var err error
	src := ff.raw
	if src == "TEST" {
		src = "~_test\\.go"
	}
	if strings.HasPrefix(src, "~") {
		ff.rx, err = regexp.Compile(src[1:])
		if err != nil {
			return fmt.Errorf("invalid file filter [%s], regexp compile error: [%w]", ff.raw, err)
		}
		return nil
	}
	/* globs */
	if strings.Contains(src, "*") {
		if fileFilterInvalidGlobRegexp.MatchString(src) {
			return fmt.Errorf("invalid file filter [%s], invalid glob pattern", ff.raw)
		}
		var rxBuild strings.Builder
		rxBuild.WriteByte('^')
		wasStar := false
		justDirGlob := false
		for _, c := range src {
			if c == '*' {
				if wasStar {
					rxBuild.WriteString(`[\s\S]*`)
					wasStar = false
					justDirGlob = true
					continue
				}
				wasStar = true
				continue
			}
			if wasStar {
				rxBuild.WriteString("[^/]*")
				wasStar = false
			}
			if strings.ContainsRune(escapeRegexSymbols, c) {
				rxBuild.WriteByte('\\')
			}
			rxBuild.WriteRune(c)
			if c == '/' && justDirGlob {
				rxBuild.WriteRune('?')
			}
			justDirGlob = false
		}
		if wasStar {
			rxBuild.WriteString("[^/]*")
		}
		rxBuild.WriteByte('$')
		ff.rx, err = regexp.Compile(rxBuild.String())
		if err != nil {
			return fmt.Errorf("invalid file filter [%s], regexp compile error after glob expand: [%w]", ff.raw, err)
		}
		return nil
	}

	// it's whole file mask, just escape dots and normalize separators
	fillRx := src
	fillRx = strings.ReplaceAll(fillRx, "\\", "/")
	fillRx = strings.ReplaceAll(fillRx, ".", `\.`)
	fillRx = "^" + fillRx + "$"
	ff.rx, err = regexp.Compile(fillRx)
	if err != nil {
		return fmt.Errorf("invalid file filter [%s], regexp compile full path: [%w]", ff.raw, err)
	}
	return nil
}
