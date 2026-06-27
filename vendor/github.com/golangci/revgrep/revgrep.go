// Package revgrep filter static analysis tools to only lines changed based on a commit reference.
package revgrep

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Checker provides APIs to filter static analysis tools to specific commits,
// such as showing only issues since last commit.
type Checker struct {
	// Patch file (unified) to read to detect lines being changed,
	// if nil revgrep will attempt to detect the VCS and generate an appropriate patch.
	// Auto-detection will search for uncommitted changes first,
	// if none found, will generate a patch from last committed change.
	// File paths within patches must be relative to current working directory.
	Patch io.Reader
	// NewFiles is a list of file names (with absolute paths) where the entire contents of the file is new.
	NewFiles []string
	// Debug sets the debug writer for additional output.
	Debug io.Writer
	// RevisionFrom check revision starting at, leave blank for auto-detection ignored if patch is set.
	RevisionFrom string
	// RevisionTo checks revision finishing at, leave blank for auto-detection ignored if patch is set.
	RevisionTo string
	// MergeBase checks revision starting at the best common ancestor, leave blank for auto-detection ignored if patch is set.
	MergeBase string
	// WholeFiles indicates that the user wishes to see all issues that comes up anywhere in any file that has been changed in this revision or patch.
	WholeFiles bool
	// Regexp to match path, line number, optional column number, and message.
	Regexp string
	// AbsPath is used to make an absolute path of an issue's filename to be relative in order to match patch file.
	// If not set, current working directory is used.
	AbsPath string

	// Calculated changes for next calls to [Checker.IsNewIssue]/[Checker.IsNew].
	changes map[string][]pos
}

// Prepare extracts a patch and changed lines.
//
// WARNING: it should only be used before an explicit call to [Checker.IsNewIssue]/[Checker.IsNew].
//
// WARNING: only [Checker.Patch], [Checker.RevisionFrom], [Checker.RevisionTo], [Checker.WholeFiles] options are used,
// the other options ([Checker.Regexp], [Checker.AbsPath]) are only used by [Checker.Check].
func (c *Checker) Prepare(ctx context.Context) error {
	err := c.loadPatch(ctx)

	c.changes = c.linesChanged()

	return err
}

// IsNew checks whether issue found by linter is new: it was found in changed lines.
//
// WARNING: it requires to call [Checker.Prepare] before call this method to load the changes from patch.
func (c *Checker) IsNew(filePath string, line int) (hunkPos int, isNew bool) {
	changes, ok := c.changes[filepath.ToSlash(filePath)]
	if !ok {
		// file wasn't changed
		return 0, false
	}

	if c.WholeFiles {
		return line, true
	}

	var (
		fpos    pos
		changed bool
	)

	// found file, see if lines matched
	for _, pos := range changes {
		if pos.lineNo == line {
			fpos = pos
			changed = true

			break
		}
	}

	if changed || changes == nil {
		// either file changed or it's a new file
		hunkPos := fpos.lineNo

		// existing file changed
		if changed {
			hunkPos = fpos.hunkPos
		}

		return hunkPos, true
	}

	return 0, false
}

// IsNewIssue checks whether issue found by linter is new: it was found in changed lines.
//
// WARNING: it requires to call [Checker.Prepare] before call this method to load the changes from patch.
func (c *Checker) IsNewIssue(i InputIssue) (hunkPos int, isNew bool) {
	return c.IsNew(i.FilePath(), i.Line())
}

// Check scans reader and writes any lines to writer that have been added in [Checker.Patch].
//
// Returns the issues written to writer when no error occurs.
//
// If no VCS could be found or other VCS errors occur,
// all issues are written to writer and an error is returned.
//
// File paths in reader must be relative to current working directory or absolute.
func (c *Checker) Check(ctx context.Context, reader io.Reader, writer io.Writer) (issues []Issue, err error) {
	errPrepare := c.Prepare(ctx)

	writeAll := errPrepare != nil

	// file.go:lineNo:colNo:message
	// colNo is optional, strip spaces before message
	lineRE := regexp.MustCompile(`(.+\.go):([0-9]+):([0-9]+)?:?\s*(.*)`)
	if c.Regexp != "" {
		lineRE, err = regexp.Compile(c.Regexp)
		if err != nil {
			return nil, fmt.Errorf("could not parse regexp: %w", err)
		}
	}

	// TODO consider lazy loading this, if there's nothing in stdin, no point
	// checking for recent changes
	c.debugf("lines changed: %+v", c.changes)

	absPath := c.AbsPath
	if absPath == "" {
		absPath, err = os.Getwd()
		if err != nil {
			errPrepare = fmt.Errorf("could not get current working directory: %w", err)
		}
	}

	// Scan each line in reader and only write those lines if lines changed
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := lineRE.FindSubmatch(scanner.Bytes())
		if line == nil {
			c.debugf("cannot parse file+line number: %s", scanner.Text())
			continue
		}

		if writeAll {
			_, _ = fmt.Fprintln(writer, scanner.Text())
			continue
		}

		// Make absolute path names relative
		path := string(line[1])
		if rel, err := filepath.Rel(absPath, path); err == nil {
			c.debugf("rewrote path from %q to %q (absPath: %q)", path, rel, absPath)
			path = rel
		}

		// Parse line number
		lno, err := strconv.ParseUint(string(line[2]), 10, 64)
		if err != nil {
			c.debugf("cannot parse line number: %q", scanner.Text())
			continue
		}

		// Parse optional column number
		var cno uint64
		if len(line[3]) > 0 {
			cno, err = strconv.ParseUint(string(line[3]), 10, 64)
			if err != nil {
				c.debugf("cannot parse column number: %q", scanner.Text())
				// Ignore this error and continue
			}
		}

		// Extract message
		msg := string(line[4])

		c.debugf("path: %q, lineNo: %v, colNo: %v, msg: %q", path, lno, cno, msg)

		simpleIssue := simpleInputIssue{filePath: path, lineNumber: int(lno)}

		hunkPos, changed := c.IsNewIssue(simpleIssue)
		if changed {
			issue := Issue{
				File:    path,
				LineNo:  int(lno),
				ColNo:   int(cno),
				HunkPos: hunkPos,
				Issue:   scanner.Text(),
				Message: msg,
			}
			issues = append(issues, issue)

			_, _ = fmt.Fprintln(writer, scanner.Text())
		} else {
			c.debugf("unchanged: %s", scanner.Text())
		}
	}

	if err := scanner.Err(); err != nil {
		errPrepare = fmt.Errorf("error reading standard input: %w", err)
	}

	return issues, errPrepare
}

func (c *Checker) debugf(format string, s ...any) {
	if c.Debug == nil {
		return
	}

	_, _ = fmt.Fprint(c.Debug, "DEBUG: ")
	_, _ = fmt.Fprintf(c.Debug, format+"\n", s...)
}

// loadPatch checks if patch is supplied, if not, retrieve from VCS.
func (c *Checker) loadPatch(ctx context.Context) error {
	if c.Patch != nil {
		return nil
	}

	option := patchOption{
		revisionFrom: c.RevisionFrom,
		revisionTo:   c.RevisionTo,
		mergeBase:    c.MergeBase,
	}

	var err error
	c.Patch, c.NewFiles, err = GitPatch(ctx, option)
	if err != nil {
		return fmt.Errorf("could not read git repo: %w", err)
	}

	if c.Patch == nil {
		return errors.New("no version control repository found")
	}

	return nil
}

// linesChanges returns a map of file names to line numbers being changed.
// If key is nil, the file has been recently added, else it contains a slice of positions that have been added.
func (c *Checker) linesChanged() map[string][]pos {
	type state struct {
		file    string
		lineNo  int   // current line number within chunk
		hunkPos int   // current line count since first @@ in file
		changes []pos // position of changes
	}

	changes := make(map[string][]pos)

	for _, file := range c.NewFiles {
		changes[file] = nil
	}

	if c.Patch == nil {
		return changes
	}

	var s state

	scanner := bufio.NewReader(c.Patch)
	var scanErr error
	for {
		lineB, isPrefix, err := scanner.ReadLine()
		if isPrefix {
			// If a single line overflowed the buffer, don't bother processing it as
			// it's likey part of a file and not relevant to the patch.
			continue
		}

		if err != nil {
			scanErr = err
			break
		}

		line := strings.TrimRight(string(lineB), "\n")

		c.debugf(line)

		s.lineNo++
		s.hunkPos++

		switch {
		case strings.HasPrefix(line, "+++ ") && len(line) > 4:
			if s.changes != nil {
				// record the last state
				changes[s.file] = s.changes
			}
			// 6 removes "+++ b/"
			s = state{file: line[6:], hunkPos: -1, changes: []pos{}}

		case strings.HasPrefix(line, "@@ "):
			//      @@ -1 +2,4 @@
			// chdr ^^^^^^^^^^^^^
			// ahdr       ^^^^
			// cstart      ^
			chdr := strings.Split(line, " ")
			ahdr := strings.Split(chdr[2], ",")

			// [1:] to remove leading plus
			cstart, err := strconv.ParseUint(ahdr[0][1:], 10, 64)
			if err != nil {
				panic(err)
			}

			s.lineNo = int(cstart) - 1 // -1 as cstart is the next line number

		case strings.HasPrefix(line, "-"):
			s.lineNo--

		case strings.HasPrefix(line, "+"):
			s.changes = append(s.changes, pos{lineNo: s.lineNo, hunkPos: s.hunkPos})
		}
	}

	if !errors.Is(scanErr, io.EOF) {
		_, _ = fmt.Fprintln(os.Stderr, "reading standard input:", scanErr)
	}

	// record the last state
	changes[s.file] = s.changes

	return changes
}

type pos struct {
	// Line number.
	lineNo int
	// Position relative to first @@ in file.
	hunkPos int
}
