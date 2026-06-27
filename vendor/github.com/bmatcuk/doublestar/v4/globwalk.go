package doublestar

import (
	"errors"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
)

// If returned from GlobWalkFunc, will cause GlobWalk to skip the current
// directory. In other words, if the current path is a directory, GlobWalk will
// not recurse into it. Otherwise, GlobWalk will skip the rest of the current
// directory.
var SkipDir = fs.SkipDir

// Callback function for GlobWalk(). If the function returns an error, GlobWalk
// will end immediately and return the same error.
type GlobWalkFunc func(path string, d fs.DirEntry) error

// GlobWalk calls the callback function `fn` for every file matching pattern.
// The syntax of pattern is the same as in Match() and the behavior is the same
// as Glob(), with regard to limitations (such as patterns containing `/./`,
// `/../`, or starting with `/`). The pattern may describe hierarchical names
// such as usr/*/bin/ed.
//
// GlobWalk may have a small performance benefit over Glob if you do not need a
// slice of matches because it can avoid allocating memory for the matches.
// Additionally, GlobWalk gives you access to the `fs.DirEntry` objects for
// each match, and lets you quit early by returning a non-nil error from your
// callback function. Like `io/fs.WalkDir`, if your callback returns `SkipDir`,
// GlobWalk will skip the current directory. This means that if the current
// path _is_ a directory, GlobWalk will not recurse into it. If the current
// path is not a directory, the rest of the parent directory will be skipped.
//
// GlobWalk ignores file system errors such as I/O errors reading directories
// by default. GlobWalk may return ErrBadPattern, reporting that the pattern is
// malformed.
//
// To enable aborting on I/O errors, the WithFailOnIOErrors option can be
// passed.
//
// Additionally, if the callback function `fn` returns an error, GlobWalk will
// exit immediately and return that error.
//
// Like Glob(), this function assumes that your pattern uses `/` as the path
// separator even if that's not correct for your OS (like Windows). If you
// aren't sure if that's the case, you can use filepath.ToSlash() on your
// pattern before calling GlobWalk().
//
// Note: users should _not_ count on the returned error,
// doublestar.ErrBadPattern, being equal to path.ErrBadPattern.
func GlobWalk(fsys fs.FS, pattern string, fn GlobWalkFunc, opts ...GlobOption) error {
	if !ValidatePattern(pattern) {
		return ErrBadPattern
	}

	g := newGlob(opts...)
	return g.doGlobWalk(fsys, pattern, true, true, fn)
}

// Actually execute GlobWalk
//   - firstSegment is true if we're in the first segment of the pattern, ie,
//     the right-most part where we can match files. If it's false, we're
//     somewhere in the middle (or at the beginning) and can only match
//     directories since there are path segments above us.
//   - beforeMeta is true if we're exploring segments before any meta
//     characters, ie, in a pattern such as `path/to/file*.txt`, the `path/to/`
//     bit does not contain any meta characters.
func (g *glob) doGlobWalk(fsys fs.FS, pattern string, firstSegment, beforeMeta bool, fn GlobWalkFunc) error {
	patternStart := indexMeta(pattern)
	if patternStart == -1 {
		// pattern doesn't contain any meta characters - does a file matching the
		// pattern exist?
		// The pattern may contain escaped wildcard characters for an exact path match.
		path := unescapeMeta(pattern)
		info, pathExists, err := g.exists(fsys, path, beforeMeta)
		if pathExists && (!firstSegment || !g.filesOnly || !info.IsDir()) {
			err = fn(path, dirEntryFromFileInfo(info))
			if err == SkipDir {
				err = nil
			}
		}
		return err
	}

	dir := "."
	splitIdx := lastIndexSlashOrAlt(pattern)
	if splitIdx != -1 {
		if pattern[splitIdx] == '}' {
			openingIdx := indexMatchedOpeningAlt(pattern[:splitIdx])
			if openingIdx == -1 {
				// if there's no matching opening index, technically Match() will treat
				// an unmatched `}` as nothing special, so... we will, too!
				splitIdx = lastIndexSlash(pattern[:splitIdx])
				if splitIdx != -1 {
					dir = pattern[:splitIdx]
					pattern = pattern[splitIdx+1:]
				}
			} else {
				// otherwise, we have to handle the alts:
				return g.globAltsWalk(fsys, pattern, openingIdx, splitIdx, firstSegment, beforeMeta, fn)
			}
		} else {
			dir = pattern[:splitIdx]
			pattern = pattern[splitIdx+1:]
		}
	}

	// if `splitIdx` is less than `patternStart`, we know `dir` has no meta
	// characters. They would be equal if they are both -1, which means `dir`
	// will be ".", and we know that doesn't have meta characters either.
	if splitIdx <= patternStart {
		return g.globDirWalk(fsys, unescapeMeta(dir), pattern, firstSegment, beforeMeta, fn)
	}

	return g.doGlobWalk(fsys, dir, false, beforeMeta, func(p string, d fs.DirEntry) error {
		if err := g.globDirWalk(fsys, p, pattern, firstSegment, false, fn); err != nil {
			return err
		}
		return nil
	})
}

// handle alts in the glob pattern - `openingIdx` and `closingIdx` are the
// indexes of `{` and `}`, respectively
func (g *glob) globAltsWalk(fsys fs.FS, pattern string, openingIdx, closingIdx int, firstSegment, beforeMeta bool, fn GlobWalkFunc) (err error) {
	var matches []DirEntryWithFullPath
	startIdx := 0
	afterIdx := closingIdx + 1
	splitIdx := lastIndexSlashOrAlt(pattern[:openingIdx])
	if splitIdx == -1 || pattern[splitIdx] == '}' {
		// no common prefix
		matches, err = g.doGlobAltsWalk(fsys, "", pattern, startIdx, openingIdx, closingIdx, afterIdx, firstSegment, beforeMeta, matches)
		if err != nil {
			return
		}
	} else {
		// our alts have a common prefix that we can process first
		startIdx = splitIdx + 1
		innerBeforeMeta := beforeMeta && !hasMetaExceptAlts(pattern[:splitIdx])
		err = g.doGlobWalk(fsys, pattern[:splitIdx], false, beforeMeta, func(p string, d fs.DirEntry) (e error) {
			matches, e = g.doGlobAltsWalk(fsys, p, pattern, startIdx, openingIdx, closingIdx, afterIdx, firstSegment, innerBeforeMeta, matches)
			return e
		})
		if err != nil {
			return
		}
	}

	skip := ""
	for _, m := range matches {
		if skip != "" {
			// Because matches are sorted, we know that descendants of the skipped
			// item must come immediately after the skipped item. If we find an item
			// that does not have a prefix matching the skipped item, we know we're
			// done skipping. I'm using strings.HasPrefix here because
			// filepath.HasPrefix has been marked deprecated (and just calls
			// strings.HasPrefix anyway). The reason it's deprecated is because it
			// doesn't handle case-insensitive paths, nor does it guarantee that the
			// prefix is actually a parent directory. Neither is an issue here: the
			// paths come from the system so their cases will match, and we guarantee
			// a parent directory by appending a slash to the prefix.
			//
			// NOTE: m.Path will always use slashes as path separators.
			if strings.HasPrefix(m.Path, skip) {
				continue
			}
			skip = ""
		}
		if err = fn(m.Path, m.Entry); err != nil {
			if err == SkipDir {
				isDir, err := g.isDir(fsys, "", m.Path, m.Entry)
				if err != nil {
					return err
				}
				if isDir {
					// append a slash to guarantee `skip` will be treated as a parent dir
					skip = m.Path + "/"
				} else {
					// Dir() calls Clean() which calls FromSlash(), so we need to convert
					// back to slashes
					skip = filepath.ToSlash(filepath.Dir(m.Path)) + "/"
				}
				err = nil
				continue
			}
			return
		}
	}

	return
}

// runs actual matching for alts
func (g *glob) doGlobAltsWalk(fsys fs.FS, d, pattern string, startIdx, openingIdx, closingIdx, afterIdx int, firstSegment, beforeMeta bool, m []DirEntryWithFullPath) (matches []DirEntryWithFullPath, err error) {
	matches = m
	matchesLen := len(m)
	patIdx := openingIdx + 1
	for patIdx < closingIdx {
		nextIdx := indexNextAlt(pattern[patIdx:closingIdx], true)
		if nextIdx == -1 {
			nextIdx = closingIdx
		} else {
			nextIdx += patIdx
		}

		alt := buildAlt(escapeMeta(d), pattern, startIdx, openingIdx, patIdx, nextIdx, afterIdx)
		err = g.doGlobWalk(fsys, alt, firstSegment, beforeMeta, func(p string, d fs.DirEntry) error {
			// insertion sort, ignoring dups
			insertIdx := matchesLen
			for insertIdx > 0 && matches[insertIdx-1].Path > p {
				insertIdx--
			}
			if insertIdx > 0 && matches[insertIdx-1].Path == p {
				// dup
				return nil
			}

			// append to grow the slice, then insert
			entry := DirEntryWithFullPath{d, p}
			matches = append(matches, entry)
			for i := matchesLen; i > insertIdx; i-- {
				matches[i] = matches[i-1]
			}
			matches[insertIdx] = entry
			matchesLen++

			return nil
		})
		if err != nil {
			return
		}

		patIdx = nextIdx + 1
	}

	return
}

func (g *glob) globDirWalk(fsys fs.FS, dir, pattern string, canMatchFiles, beforeMeta bool, fn GlobWalkFunc) (e error) {
	if pattern == "" {
		if !canMatchFiles || !g.filesOnly {
			// pattern can be an empty string if the original pattern ended in a
			// slash, in which case, we should just return dir, but only if it
			// actually exists and it's a directory (or a symlink to a directory)
			info, isDir, err := g.isPathDir(fsys, dir, beforeMeta)
			if err != nil {
				return err
			}
			if isDir {
				e = fn(dir, dirEntryFromFileInfo(info))
				if e == SkipDir {
					e = nil
				}
			}
		}
		return
	}

	if pattern == "**" {
		// `**` can match *this* dir
		info, dirExists, err := g.exists(fsys, dir, beforeMeta)
		if err != nil {
			return err
		}
		if !dirExists || !info.IsDir() {
			return nil
		}
		if !canMatchFiles || !g.filesOnly {
			if e = fn(dir, dirEntryFromFileInfo(info)); e != nil {
				if e == SkipDir {
					e = nil
				}
				return
			}
		}
		return g.globDoubleStarWalk(fsys, dir, canMatchFiles, fn)
	}

	dirs, err := fs.ReadDir(fsys, dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return g.handlePatternNotExist(beforeMeta)
		}
		return g.forwardErrIfFailOnIOErrors(err)
	}

	var matched bool
	checkForHidden := g.noHidden && couldUnintentionallyMatchHidden(pattern)
	for _, info := range dirs {
		name := info.Name()

		// Skip hidden files when noHidden is set
		if checkForHidden {
			isHidden, err := isHiddenPath(name, info)
			if e = g.forwardErrIfFailOnIOErrors(err); e != nil {
				return
			}
			if isHidden {
				continue
			}
		}

		matched, e = matchWithSeparator(pattern, name, '/', false, g.caseInsensitive)
		if e != nil {
			return
		}
		if matched {
			matched = canMatchFiles
			if !matched || g.filesOnly {
				matched, e = g.isDir(fsys, dir, name, info)
				if e != nil {
					return e
				}
				if canMatchFiles {
					// if we're here, it's because g.filesOnly
					// is set and we don't want directories
					matched = !matched
				}
			}
			if matched {
				if e = fn(path.Join(dir, name), info); e != nil {
					if e == SkipDir {
						e = nil
					}
					return
				}
			}
		}
	}

	return
}

// recursively walk files/directories in a directory
func (g *glob) globDoubleStarWalk(fsys fs.FS, dir string, canMatchFiles bool, fn GlobWalkFunc) (e error) {
	dirs, err := fs.ReadDir(fsys, dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// This function is only ever called after we know the top-most directory
			// exists, so, if we ever get here, we know we'll never return
			// ErrPatternNotExist.
			return nil
		}
		return g.forwardErrIfFailOnIOErrors(err)
	}

	for _, info := range dirs {
		name := info.Name()

		// Skip hidden files/directories when noHidden is set
		if g.noHidden {
			isHidden, err := isHiddenPath(name, info)
			if e = g.forwardErrIfFailOnIOErrors(err); e != nil {
				return
			}
			if isHidden {
				continue
			}
		}

		isDir, err := g.isDir(fsys, dir, name, info)
		if err != nil {
			return err
		}

		if isDir {
			p := path.Join(dir, name)
			if !canMatchFiles || !g.filesOnly {
				// `**` can match *this* dir, so add it
				if e = fn(p, info); e != nil {
					if e == SkipDir {
						e = nil
						continue
					}
					return
				}
			}
			if e = g.globDoubleStarWalk(fsys, p, canMatchFiles, fn); e != nil {
				return
			}
		} else if canMatchFiles {
			if e = fn(path.Join(dir, name), info); e != nil {
				if e == SkipDir {
					e = nil
				}
				return
			}
		}
	}

	return
}

type DirEntryFromFileInfo struct {
	fi fs.FileInfo
}

func (d *DirEntryFromFileInfo) Name() string {
	return d.fi.Name()
}

func (d *DirEntryFromFileInfo) IsDir() bool {
	return d.fi.IsDir()
}

func (d *DirEntryFromFileInfo) Type() fs.FileMode {
	return d.fi.Mode().Type()
}

func (d *DirEntryFromFileInfo) Info() (fs.FileInfo, error) {
	return d.fi, nil
}

func dirEntryFromFileInfo(fi fs.FileInfo) fs.DirEntry {
	return &DirEntryFromFileInfo{fi}
}

type DirEntryWithFullPath struct {
	Entry fs.DirEntry
	Path  string
}

func hasMetaExceptAlts(s string) bool {
	var c byte
	l := len(s)
	for i := 0; i < l; i++ {
		c = s[i]
		if c == '*' || c == '?' || c == '[' {
			return true
		} else if c == '\\' {
			// skip next byte
			i++
		}
	}
	return false
}
