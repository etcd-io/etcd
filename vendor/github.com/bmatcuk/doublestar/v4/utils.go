package doublestar

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// SplitPattern is a utility function. Given a pattern, SplitPattern will
// return two strings: the first string is everything up to the last slash
// (`/`) that appears _before_ any unescaped "meta" characters (ie, `*?[{`).
// The second string is everything after that slash. For example, given the
// pattern:
//
//	../../path/to/meta*/**
//	             ^----------- split here
//
// SplitPattern returns "../../path/to" and "meta*/**". This is useful for
// initializing os.DirFS() to call Glob() because Glob() will silently fail if
// your pattern includes `/./` or `/../`. For example:
//
//	base, pattern := SplitPattern("../../path/to/meta*/**")
//	fsys := os.DirFS(base)
//	matches, err := Glob(fsys, pattern)
//
// If SplitPattern cannot find somewhere to split the pattern (for example,
// `meta*/**`), it will return "." and the unaltered pattern (`meta*/**` in
// this example).
//
// Note that SplitPattern will also unescape any meta characters in the
// returned base string, so that it can be passed straight to os.DirFS().
//
// Of course, it is your responsibility to decide if the returned base path is
// "safe" in the context of your application. Perhaps you could use Match() to
// validate against a list of approved base directories?
func SplitPattern(p string) (base, pattern string) {
	base = "."
	pattern = p

	splitIdx := -1
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c == '\\' {
			i++
		} else if c == '/' {
			splitIdx = i
		} else if c == '*' || c == '?' || c == '[' || c == '{' {
			break
		}
	}

	if splitIdx == 0 {
		return "/", p[1:]
	} else if splitIdx > 0 {
		return unescapeMeta(p[:splitIdx]), p[splitIdx+1:]
	}

	return
}

// FilepathGlob returns the names of all files matching pattern or nil if there
// is no matching file. The syntax of pattern is the same as in Match(). The
// pattern may describe hierarchical names such as usr/*/bin/ed.
//
// FilepathGlob ignores file system errors such as I/O errors reading
// directories by default. The only possible returned error is ErrBadPattern,
// reporting that the pattern is malformed.
//
// To enable aborting on I/O errors, the WithFailOnIOErrors option can be
// passed.
//
// Note: FilepathGlob is a convenience function that is meant as a drop-in
// replacement for `path/filepath.Glob()` for users who don't need the
// complication of io/fs. Basically, it:
//   - Runs `filepath.Clean()` and `ToSlash()` on the pattern
//   - Runs `SplitPattern()` to get a base path and a pattern to Glob
//   - Creates an FS object from the base path and `Glob()s` on the pattern
//   - Joins the base path with all of the matches from `Glob()`
//
// Returned paths will use the system's path separator, just like
// `filepath.Glob()`.
//
// Note: the returned error doublestar.ErrBadPattern is not equal to
// filepath.ErrBadPattern.
func FilepathGlob(pattern string, opts ...GlobOption) (matches []string, err error) {
	if pattern == "" {
		// special case to match filepath.Glob behavior
		g := newGlob(opts...)
		if g.failOnIOErrors {
			// match doublestar.Glob behavior here
			return nil, os.ErrInvalid
		}
		return nil, nil
	}

	pattern = filepath.Clean(pattern)
	pattern = filepath.ToSlash(pattern)
	base, f := SplitPattern(pattern)
	if f == "" || f == "." || f == ".." {
		// some special cases to match filepath.Glob behavior
		if !ValidatePathPattern(pattern) {
			return nil, ErrBadPattern
		}

		if filepath.Separator != '\\' {
			pattern = unescapeMeta(pattern)
		}

		if _, err = os.Lstat(pattern); err != nil {
			g := newGlob(opts...)
			if errors.Is(err, os.ErrNotExist) {
				return nil, g.handlePatternNotExist(true)
			}
			return nil, g.forwardErrIfFailOnIOErrors(err)
		}
		return []string{filepath.FromSlash(pattern)}, nil
	}

	fs := os.DirFS(base)
	if matches, err = Glob(fs, f, opts...); err != nil {
		return nil, err
	}
	for i := range matches {
		// use path.Join because we used ToSlash above to ensure our paths are made
		// of forward slashes, no matter what the system uses
		matches[i] = filepath.FromSlash(path.Join(base, matches[i]))
	}
	return
}

// Finds the next comma, but ignores any commas that appear inside nested `{}`.
// Assumes that each opening bracket has a corresponding closing bracket.
func indexNextAlt(s string, allowEscaping bool) int {
	alts := 1
	l := len(s)
	for i := 0; i < l; i++ {
		if allowEscaping && s[i] == '\\' {
			// skip next byte
			i++
		} else if s[i] == '{' {
			alts++
		} else if s[i] == '}' {
			alts--
		} else if s[i] == ',' && alts == 1 {
			return i
		}
	}
	return -1
}

var escapeMetaReplacer = strings.NewReplacer("*", "\\*", "?", "\\?", "[", "\\[", "]", "\\]", "{", "\\{", "}", "\\}")

// Escapes meta characters (*?[]{})
func escapeMeta(path string) string {
	return escapeMetaReplacer.Replace(path)
}

var unescapeMetaReplacer = strings.NewReplacer("\\*", "*", "\\?", "?", "\\[", "[", "\\]", "]", "\\{", "{", "\\}", "}")

// Unescapes meta characters (*?[]{})
func unescapeMeta(pattern string) string {
	return unescapeMetaReplacer.Replace(pattern)
}
