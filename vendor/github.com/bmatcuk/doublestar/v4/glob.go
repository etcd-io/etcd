package doublestar

import (
	"errors"
	"io/fs"
	"path"
)

// Glob returns the names of all files matching pattern or nil if there is no
// matching file. The syntax of pattern is the same as in Match(). The pattern
// may describe hierarchical names such as usr/*/bin/ed.
//
// Glob ignores file system errors such as I/O errors reading directories by
// default. The only possible returned error is ErrBadPattern, reporting that
// the pattern is malformed.
//
// To enable aborting on I/O errors, the WithFailOnIOErrors option can be
// passed.
//
// Note: this is meant as a drop-in replacement for io/fs.Glob(). Like
// io/fs.Glob(), this function assumes that your pattern uses `/` as the path
// separator even if that's not correct for your OS (like Windows). If you
// aren't sure if that's the case, you can use filepath.ToSlash() on your
// pattern before calling Glob().
//
// Like `io/fs.Glob()`, patterns containing `/./`, `/../`, or starting with `/`
// will return no results and no errors. You can use SplitPattern to divide a
// pattern into a base path (to initialize an `FS` object) and pattern.
//
// Note: users should _not_ count on the returned error,
// doublestar.ErrBadPattern, being equal to path.ErrBadPattern.
func Glob(fsys fs.FS, pattern string, opts ...GlobOption) ([]string, error) {
	if !ValidatePattern(pattern) {
		return nil, ErrBadPattern
	}

	g := newGlob(opts...)

	if hasMidDoubleStar(pattern) {
		// If the pattern has a `**` anywhere but the very end, GlobWalk is more
		// performant because it can get away with less allocations. If the pattern
		// ends in a `**`, both methods are pretty much the same, but Glob has a
		// _very_ slight advantage because of lower function call overhead.
		var matches []string
		err := g.doGlobWalk(fsys, pattern, true, true, func(p string, d fs.DirEntry) error {
			matches = append(matches, p)
			return nil
		})
		return matches, err
	}
	return g.doGlob(fsys, pattern, nil, true, true)
}

// Does the actual globbin'
//   - firstSegment is true if we're in the first segment of the pattern, ie,
//     the right-most part where we can match files. If it's false, we're
//     somewhere in the middle (or at the beginning) and can only match
//     directories since there are path segments above us.
//   - beforeMeta is true if we're exploring segments before any meta
//     characters, ie, in a pattern such as `path/to/file*.txt`, the `path/to/`
//     bit does not contain any meta characters.
func (g *glob) doGlob(fsys fs.FS, pattern string, m []string, firstSegment, beforeMeta bool) (matches []string, err error) {
	matches = m
	patternStart := indexMeta(pattern)
	if patternStart == -1 {
		// pattern doesn't contain any meta characters - does a file matching the
		// pattern exist?
		// The pattern may contain escaped wildcard characters for an exact path match.
		path := unescapeMeta(pattern)
		pathInfo, pathExists, pathErr := g.exists(fsys, path, beforeMeta)
		if pathErr != nil {
			return nil, pathErr
		}

		if pathExists && (!firstSegment || !g.filesOnly || !pathInfo.IsDir()) {
			matches = append(matches, path)
		}

		return
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
				return g.globAlts(fsys, pattern, openingIdx, splitIdx, matches, firstSegment, beforeMeta)
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
		return g.globDir(fsys, unescapeMeta(dir), pattern, matches, firstSegment, beforeMeta)
	}

	var dirs []string
	dirs, err = g.doGlob(fsys, dir, matches, false, beforeMeta)
	if err != nil {
		return
	}
	for _, d := range dirs {
		matches, err = g.globDir(fsys, d, pattern, matches, firstSegment, false)
		if err != nil {
			return
		}
	}

	return
}

// handle alts in the glob pattern - `openingIdx` and `closingIdx` are the
// indexes of `{` and `}`, respectively
func (g *glob) globAlts(fsys fs.FS, pattern string, openingIdx, closingIdx int, m []string, firstSegment, beforeMeta bool) (matches []string, err error) {
	matches = m

	var dirs []string
	startIdx := 0
	afterIdx := closingIdx + 1
	splitIdx := lastIndexSlashOrAlt(pattern[:openingIdx])
	if splitIdx == -1 || pattern[splitIdx] == '}' {
		// no common prefix
		dirs = []string{""}
	} else {
		// our alts have a common prefix that we can process first
		dirs, err = g.doGlob(fsys, pattern[:splitIdx], matches, false, beforeMeta)
		if err != nil {
			return
		}

		startIdx = splitIdx + 1
	}

	for _, d := range dirs {
		patIdx := openingIdx + 1
		altResultsStartIdx := len(matches)
		thisResultStartIdx := altResultsStartIdx
		for patIdx < closingIdx {
			nextIdx := indexNextAlt(pattern[patIdx:closingIdx], true)
			if nextIdx == -1 {
				nextIdx = closingIdx
			} else {
				nextIdx += patIdx
			}

			alt := buildAlt(escapeMeta(d), pattern, startIdx, openingIdx, patIdx, nextIdx, afterIdx)
			matches, err = g.doGlob(fsys, alt, matches, firstSegment, beforeMeta)
			if err != nil {
				return
			}

			matchesLen := len(matches)
			if altResultsStartIdx != thisResultStartIdx && thisResultStartIdx != matchesLen {
				// Alts can result in matches that aren't sorted, or, worse, duplicates
				// (consider the trivial pattern `path/to/{a,*}`). Since doGlob returns
				// sorted results, we can do a sort of in-place merge and remove
				// duplicates. But, we only need to do this if this isn't the first alt
				// (ie, `altResultsStartIdx != thisResultsStartIdx`) and if the latest
				// alt actually added some matches (`thisResultStartIdx !=
				// len(matches)`)
				matches = sortAndRemoveDups(matches, altResultsStartIdx, thisResultStartIdx, matchesLen)

				// length of matches may have changed
				thisResultStartIdx = len(matches)
			} else {
				thisResultStartIdx = matchesLen
			}

			patIdx = nextIdx + 1
		}
	}

	return
}

// find files/subdirectories in the given `dir` that match `pattern`
func (g *glob) globDir(fsys fs.FS, dir, pattern string, matches []string, canMatchFiles, beforeMeta bool) (m []string, e error) {
	m = matches

	if pattern == "" {
		if !canMatchFiles || !g.filesOnly {
			// pattern can be an empty string if the original pattern ended in a
			// slash, in which case, we should just return dir, but only if it
			// actually exists and it's a directory (or a symlink to a directory)
			_, isDir, err := g.isPathDir(fsys, dir, beforeMeta)
			if err != nil {
				return nil, err
			}
			if isDir {
				m = append(m, dir)
			}
		}
		return
	}

	if pattern == "**" {
		return g.globDoubleStar(fsys, dir, m, canMatchFiles, beforeMeta)
	}

	dirs, err := fs.ReadDir(fsys, dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			e = g.handlePatternNotExist(beforeMeta)
		} else {
			e = g.forwardErrIfFailOnIOErrors(err)
		}
		return
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
					return
				}
				if canMatchFiles {
					// if we're here, it's because g.filesOnly
					// is set and we don't want directories
					matched = !matched
				}
			}
			if matched {
				m = append(m, path.Join(dir, name))
			}
		}
	}

	return
}

func (g *glob) globDoubleStar(fsys fs.FS, dir string, matches []string, canMatchFiles, beforeMeta bool) ([]string, error) {
	dirs, err := fs.ReadDir(fsys, dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return matches, g.handlePatternNotExist(beforeMeta)
		} else {
			return matches, g.forwardErrIfFailOnIOErrors(err)
		}
	}

	if !g.filesOnly {
		// `**` can match *this* dir, so add it
		matches = append(matches, dir)
	}

	for _, info := range dirs {
		name := info.Name()

		// Skip hidden files/directories when noHidden is set
		if g.noHidden {
			isHidden, err := isHiddenPath(name, info)
			if err = g.forwardErrIfFailOnIOErrors(err); err != nil {
				return nil, err
			}
			if isHidden {
				continue
			}
		}

		isDir, err := g.isDir(fsys, dir, name, info)
		if err != nil {
			return nil, err
		}
		if isDir {
			matches, err = g.globDoubleStar(fsys, path.Join(dir, name), matches, canMatchFiles, false)
			if err != nil {
				return nil, err
			}
		} else if canMatchFiles {
			matches = append(matches, path.Join(dir, name))
		}
	}

	return matches, nil
}

// Returns true if the pattern has a doublestar in the middle of the pattern.
// In this case, GlobWalk is faster because it can get away with less
// allocations. However, Glob has a _very_ slight edge if the pattern ends in
// `**`.
func hasMidDoubleStar(p string) bool {
	// subtract 3: 2 because we want to return false if the pattern ends in `**`
	// (Glob is _very_ slightly faster in that case), and the extra 1 because our
	// loop checks p[i] and p[i+1].
	l := len(p) - 3
	for i := 0; i < l; i++ {
		if p[i] == '\\' {
			// escape next byte
			i++
		} else if p[i] == '*' && p[i+1] == '*' {
			return true
		}
	}
	return false
}

// Returns the index of the first unescaped meta character, or negative 1.
func indexMeta(s string) int {
	var c byte
	l := len(s)
	for i := 0; i < l; i++ {
		c = s[i]
		if c == '*' || c == '?' || c == '[' || c == '{' {
			return i
		} else if c == '\\' {
			// skip next byte
			i++
		}
	}
	return -1
}

// Returns the index of the last unescaped slash or closing alt (`}`) in the
// string, or negative 1.
func lastIndexSlashOrAlt(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if (s[i] == '/' || s[i] == '}') && (i == 0 || s[i-1] != '\\') {
			return i
		}
	}
	return -1
}

// Returns the index of the last unescaped slash in the string, or negative 1.
func lastIndexSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' && (i == 0 || s[i-1] != '\\') {
			return i
		}
	}
	return -1
}

// Assuming the byte after the end of `s` is a closing `}`, this function will
// find the index of the matching `{`. That is, it'll skip over any nested `{}`
// and account for escaping.
func indexMatchedOpeningAlt(s string) int {
	alts := 1
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '}' && (i == 0 || s[i-1] != '\\') {
			alts++
		} else if s[i] == '{' && (i == 0 || s[i-1] != '\\') {
			if alts--; alts == 0 {
				return i
			}
		}
	}
	return -1
}

// Returns true if the path exists
func (g *glob) exists(fsys fs.FS, name string, beforeMeta bool) (fs.FileInfo, bool, error) {
	// name might end in a slash, but Stat doesn't like that
	namelen := len(name)
	if namelen > 1 && name[namelen-1] == '/' {
		name = name[:namelen-1]
	}

	info, err := fs.Stat(fsys, name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, g.handlePatternNotExist(beforeMeta)
	}
	return info, err == nil, g.forwardErrIfFailOnIOErrors(err)
}

// Returns true if the path exists and is a directory or a symlink to a
// directory
func (g *glob) isPathDir(fsys fs.FS, name string, beforeMeta bool) (fs.FileInfo, bool, error) {
	info, err := fs.Stat(fsys, name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, g.handlePatternNotExist(beforeMeta)
	}
	return info, err == nil && info.IsDir(), g.forwardErrIfFailOnIOErrors(err)
}

// Returns whether or not the given DirEntry is a directory. If the DirEntry
// represents a symbolic link, the link is followed by running fs.Stat() on
// `path.Join(dir, name)` (if dir is "", name will be used without joining)
func (g *glob) isDir(fsys fs.FS, dir, name string, info fs.DirEntry) (bool, error) {
	if !g.noFollow && (info.Type()&fs.ModeSymlink) > 0 {
		p := name
		if dir != "" {
			p = path.Join(dir, name)
		}
		finfo, err := fs.Stat(fsys, p)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// this function is only ever called while expanding a glob, so it can
				// never return ErrPatternNotExist
				return false, nil
			}
			return false, g.forwardErrIfFailOnIOErrors(err)
		}
		return finfo.IsDir(), nil
	}
	return info.IsDir(), nil
}

// Builds a string from an alt
func buildAlt(prefix, pattern string, startIdx, openingIdx, currentIdx, nextIdx, afterIdx int) string {
	// pattern:
	//   ignored/start{alts,go,here}remaining - len = 36
	//           |    |     | |     ^--- afterIdx   = 27
	//           |    |     | \--------- nextIdx    = 21
	//           |    |     \----------- currentIdx = 19
	//           |    \----------------- openingIdx = 13
	//           \---------------------- startIdx   = 8
	//
	// result:
	//   prefix/startgoremaining - len = 7 + 5 + 2 + 9 = 23
	var buf []byte
	patLen := len(pattern)
	size := (openingIdx - startIdx) + (nextIdx - currentIdx) + (patLen - afterIdx)
	if prefix != "" && prefix != "." {
		buf = make([]byte, 0, size+len(prefix)+1)
		buf = append(buf, prefix...)
		buf = append(buf, '/')
	} else {
		buf = make([]byte, 0, size)
	}
	buf = append(buf, pattern[startIdx:openingIdx]...)
	buf = append(buf, pattern[currentIdx:nextIdx]...)
	if afterIdx < patLen {
		buf = append(buf, pattern[afterIdx:]...)
	}
	return string(buf)
}

// Running alts can produce results that are not sorted, and, worse, can cause
// duplicates (consider the trivial pattern `path/to/{a,*}`). Since we know
// each run of doGlob is sorted, we can basically do the "merge" step of a
// merge sort in-place.
func sortAndRemoveDups(matches []string, idx1, idx2, l int) []string {
	var tmp string
	for ; idx1 < idx2; idx1++ {
		if matches[idx1] < matches[idx2] {
			// order is correct
			continue
		} else if matches[idx1] > matches[idx2] {
			// need to swap and then re-sort matches above idx2
			tmp = matches[idx1]
			matches[idx1] = matches[idx2]

			shft := idx2 + 1
			for ; shft < l && matches[shft] < tmp; shft++ {
				matches[shft-1] = matches[shft]
			}
			matches[shft-1] = tmp
		} else {
			// duplicate - shift matches above idx2 down one and decrement l
			for shft := idx2 + 1; shft < l; shft++ {
				matches[shft-1] = matches[shft]
			}
			if l--; idx2 == l {
				// nothing left to do... matches[idx2:] must have been full of dups
				break
			}
		}
	}
	return matches[:l]
}
