// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains an implementation of the greedy longest common
// subsequence/shortest edit script (LCS/SES) algorithm described in
// Eugene W. Myers, "An O(ND) Difference Algorithm and Its Variations"
//
// It also contains support for creating unified diffs (i.e., patch files).
// The unified diff format is documented in the POSIX standard (IEEE 1003.1),
// "diff - compare two files", section: "Diff -u or -U Output Format"
// http://pubs.opengroup.org/onlinepubs/9699919799/utilities/diff.html

package text

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"
)

/* -=-=- Myers Diff Algorithm Implementation -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

// Diff creates an EditSet containing the minimum number of line additions and
// deletions necessary to change a into b.  Typically, both a and b will be
// slices containing \n-terminated lines of a larger string, although it is
// also possible compute character-by-character diffs by splitting a string on
// UTF-8 boundaries.  The resulting EditSet is constructed so that it can be
// applied to the string produced by strings.Join(a, "").
//
// Every edit in the resulting EditSet starts at an offset corresponding to the
// first character on a line.  Every edit in the EditSet is either (1) a
// deletion, i.e., its length is the length of the current line and its
// replacement text is the empty string, or (2) an addition, i.e., its length
// is 0 and its replacement text is a single line to insert.
//
// The implementation follows the pseudocode in Myers' paper (cited above)
// fairly closely.
func Diff(a []string, b []string) *EditSet {
	n := len(a)
	m := len(b)
	max := m + n
	if n == 0 && m == 0 {
		return &EditSet{}
	} else if n == 0 {
		result := &EditSet{}
		replacement := strings.Join(b, "")
		if replacement != "" {
			result.Add(&Extent{0, 0}, replacement)
		}
		return result
	} else if m == 0 {
		result := &EditSet{}
		length := len(strings.Join(a, ""))
		if length > 0 {
			result.Add(&Extent{0, length}, "")
		}
		return result
	}
	vs := make([][]int, 0, max)
	v := make([]int, 2*max, 2*max)
	offset := max
	v[offset+1] = 0
	for d := 0; d <= max; d++ {
		for k := -d; k <= d; k += 2 {
			var x, y int
			if k == -d || k != d && v[offset+k-1] < v[offset+k+1] {
				// Vertical edge
				x = v[offset+k+1]
			} else {
				// Horizontal edge
				x = v[offset+k-1] + 1
			}
			y = x - k
			for x < n && y < m && a[x] == b[y] {
				x = x + 1
				y = y + 1
			}
			v[offset+k] = x
			if x >= n && y >= m {
				// length of SES is D
				vs = append(vs, v)
				//return constructEditSet(a, b, vs)
				edits := &EditSet{}
				constructEditSet(a, b, vs, edits, n-m)
				return edits
			}
		}
		vCopy := make([]int, len(v))
		copy(vCopy, v)
		vs = append(vs, vCopy)
	}
	panic("Length of SES longer than max (internal error)")
}

// constructEditSet uses the matrixtargetvs (computed by Diff) to compute a
// sequence of deletions and additions.
func constructEditSet(a, b []string, vs [][]int, edits *EditSet, k int) {
	n := len(a)
	m := len(b)
	max := m + n
	offset := max

	for len(vs) > 1 {
		v := vs[len(vs)-1]

		x := v[offset+k]
		y := x - k

		d := len(vs)
		if k == -d+1 || k != d-1 && v[offset+k-1] < v[offset+k+1] {
			// Insert
			k++
			prevx := v[offset+k]
			prevy := prevx - k

			charsToCopy := y - prevy - 1
			insertOffset := x - charsToCopy
			ol := &Extent{offsetOfString(insertOffset, a), 0}
			copyOffset := y - charsToCopy - 1
			replaceWith := b[copyOffset : copyOffset+1]
			replacement := strings.Join(replaceWith, "")
			if len(replacement) > 0 {
				edits.Add(ol, replacement)
			}
		} else {
			// Delete
			k--
			prevx := v[offset+k]

			charsToCopy := x - prevx - 1
			deleteOffset := x - charsToCopy - 1
			ol := &Extent{
				offsetOfString(deleteOffset, a),
				len(a[deleteOffset])}
			replaceWith := ""
			if ol.Length > 0 {
				edits.Add(ol, replaceWith)
			}
		}

		vs = vs[:len(vs)-1]
	}
}

// offsetOfString returns the byte offset of the substring ss[indextarget in the
// string strings.Join(ss, "")
func offsetOfString(index int, ss []string) int {
	var result int
	for i := 0; i < index; i++ {
		result += len(ss[i])
	}
	return result
}

/* -=-=- Unified Diff Support =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

// numCtxLines is the number of leading/trailing context lines in a unified diff
const numCtxLines int = 3

// A Patch is an object representing a unified diff.  It can be created from an
// EditSet by invoking the CreatePatch method.  To get the contents of the
// unified diff, invoke the Write method.
type Patch struct {
	filename string
	hunks    []*hunk
}

// IsEmpty returns true iff this patch contains no hunks
func (p *Patch) IsEmpty() bool {
	return len(p.hunks) == 0
}

// add appends a hunk to this patch.  It is the caller's responsibility to
// ensure that hunks are added in the correct order.
func (p *Patch) add(hunk *hunk) {
	p.hunks = append(p.hunks, hunk)
}

// Write writes a unified diff to the given io.Writer.  The given filenames
// are used in the diff output.
func (p *Patch) Write(origFile, newFile string, origTime, newTime time.Time, out io.Writer) error {
	if !p.IsEmpty() {
		layout := ""
		if !origTime.IsZero() || !newTime.IsZero() {
			layout = "  2006-01-02 15:04:05 -0700"
		}
		fmt.Fprintf(out, "--- %s%s\n+++ %s%s\n",
			origFile, origTime.Format(layout),
			newFile, newTime.Format(layout))
		lineOffset := 0
		for _, hunk := range p.hunks {
			adjust, err := writeDiffHunk(hunk, lineOffset, out)
			if err != nil {
				return err
			}
			lineOffset += adjust
		}
	}
	return nil
}

// writeDiffHunk writes a single hunk in unified diff format.  If the
// edits in that hunk add lines, it returns the number of lines added; if the
// edits delete lines, it returns a negative number indicating the number of
// lines deleted (0 - number of lines deleted).  If the edits in the hunk do
// not change the number of lines, returns 0.
func writeDiffHunk(h *hunk, outputLineOffset int, out io.Writer) (int, error) {
	// Determine the lines in this hunk before and after applying edits
	origLines, newLines, err := computeLines(h)
	if err != nil {
		return 0, err
	}

	// Write the unified diff header
	numOrigLines := lenWithoutLastIfEmpty(origLines)
	numNewLines := lenWithoutLastIfEmpty(newLines)
	if _, err = fmt.Fprintf(out, "@@ -%d,%d +%d,%d @@\n",
		h.startLine, numOrigLines,
		h.startLine+outputLineOffset, numNewLines); err != nil {
		return 0, err
	}

	// Create an iterator that will traverse deletions and additions
	it := Diff(origLines, newLines).newEditIter()

	// For each line in the original file, add one or more lines to the
	// unified diff output
	offset := 0
	for i, line := range origLines {
		if it.edit() == nil || it.edit().Offset > offset {
			// This line was not affected by any edits
			if i < len(origLines)-1 || line != "" {
				fmt.Fprintf(out, " %s", origLines[i])
			}
		} else {
			// This line was deleted (and possibly replaced by a
			// different line), or one or more lines were inserted
			// before this line
			deleted := false
			for it.edit() != nil && it.edit().Offset == offset ||
				it.edit() != nil && i == len(origLines)-1 {
				edit := it.edit()
				if edit.Length > 0 {
					// Delete line
					line := origLines[i]
					fmt.Fprintf(out, "-%s", line)
					if !strings.HasSuffix(line, "\n") {
						fmt.Fprintf(out, "\n\\ No newline at end of file\n")
					}
					deleted = true
				} else if edit.replacement != "" {
					// Insert line
					repl := edit.replacement
					fmt.Fprintf(out, "+%s", repl)
					if !strings.HasSuffix(repl, "\n") {
						fmt.Fprintf(out, "\n\\ No newline at end of file\n")
					}

				}
				it.moveToNextEdit()
			}
			if !deleted {
				if i < len(origLines)-1 || line != "" {
					fmt.Fprintf(out, " %s", origLines[i])
				}
			}
		}
		offset += len(line)
	}
	return numNewLines - numOrigLines, nil
}

// If the last string in the slice is the empty string, returns len(ss)-1;
// otherwise, returns len(ss).
func lenWithoutLastIfEmpty(ss []string) int {
	if len(ss) > 0 && ss[len(ss)-1] == "" {
		return len(ss) - 1
	}
	return len(ss)
}

// computeLines computes the text that will result from applying the edits in
// this hunk, then returns both the original text and the new text split into
// lines on \n boundaries.  It returns a non-nil error if the edits in the hunk
// cannot be applied.
func computeLines(h *hunk) (origLines []string, newLines []string, err error) {
	hunk := h.hunk.String()
	newText, err := ApplyToString(&EditSet{edits: h.edits}, hunk)
	if err != nil {
		return
	}

	origLines = strings.SplitAfter(hunk, "\n")
	newLines = strings.SplitAfter(newText, "\n")

	numOrig := len(origLines)
	numNew := len(newLines)
	trailingCtxLines := 0
	for i, n := 0, min(numOrig, numNew); i < n; i++ {
		if origLines[numOrig-i-1] == newLines[numNew-i-1] {
			trailingCtxLines++
		} else {
			break
		}
	}
	linesToRemove := max(trailingCtxLines-numCtxLines, 0)

	origLines = origLines[:numOrig-linesToRemove]
	newLines = newLines[:numNew-linesToRemove]
	return
}

// min returns the minimum of two integers.
func min(m, n int) int {
	if m < n {
		return m
	}
	return n
}

// maxtargetreturns the maximum of two integers.
func max(m, n int) int {
	if m > n {
		return m
	}
	return n
}

// A hunk represents a single hunk in a unified diff.  A hunk consists of all
// of the edits that affect a particular region of a file.  Typically, a hunk
// is written with numCtxLines lines of context preceding and following
// the hunk.  So, edits are grouped: if there are more than 2*numCtxLines+1
// lines between two edits, they should be in separate hunks.  Otherwise, the
// two edits should be in the same hunk.
type hunk struct {
	startOffset int          // Offset of this hunk in the original file
	startLine   int          // 1-based line number of this hunk
	numLines    int          // Number of lines modified by this hunk
	hunk        bytes.Buffer // Affected bytes from the original file
	edits       []edit       // Edits to be applied to hunk
}

// addLine adds a single line of text to the hunk.
func (h *hunk) addLine(line string) {
	h.hunk.WriteString(line)
	h.numLines++
}

// addEdit appends a single edit to the hunk.  It is the caller's
// responsibility to ensure that edits are added in sorted order.
func (h *hunk) addEdit(e *edit) {
	h.edits = append(h.edits, e.RelativeToOffset(h.startOffset))
}

// A lineRdr reads lines, one at a time, from an io.Reader, keeping track of
// the 0-based offset and 1-based line number of the line.  It also keeps
// track of the previous numCtxLines lines that were read.  (This is used to
// create leading context for a unified diff hunk.)
type lineRdr struct {
	reader          *bufio.Reader
	line            string
	lineOffset      int
	lineNum         int
	err             error
	leadingCtxLines []string
}

// newLineRdr creates a new lineRdr that reads from the given io.Reader.
func newLineRdr(in io.Reader) *lineRdr {
	return &lineRdr{reader: bufio.NewReader(in)}
}

// readLine reads a single line from the wrapped io.Reader.  When the end of
// the input is reached, it returns io.EOF.
func (l *lineRdr) readLine() error {
	if l.lineNum > 0 {
		if len(l.leadingCtxLines) == numCtxLines {
			l.leadingCtxLines = l.leadingCtxLines[1:]
		}
		l.leadingCtxLines = append(l.leadingCtxLines, l.line)
	}
	l.lineOffset += len(l.line)
	l.lineNum++
	l.line, l.err = l.reader.ReadString('\n')
	return l.err
}

// offsetPastEnd returns the 0-based offset of the first character on the line
// following the line that was read, or the length of the file if the end was
// reached.
func (l *lineRdr) offsetPastEnd() int {
	return l.lineOffset + len(l.line)
}

// editAddsToStart returns true iff the given edit adds characters at the
// beginning of this line without modifying or deleting any characters in the
// line.
func (l *lineRdr) editAddsToStart(e *edit) bool {
	if e == nil {
		return false
	}
	return e.Offset == l.lineOffset && e.Length == 0
}

// currentLineIsAffectedBy returns true iff the given edit adds characters to,
// modifies, or deletes characters from the line that was most recently read.
func (l *lineRdr) currentLineIsAffectedBy(e *edit) bool {
	if e == nil {
		return false
	} else if l.err == io.EOF {
		return e.OffsetPastEnd() >= l.lineOffset
	} else {
		return e.Offset < l.offsetPastEnd() &&
			e.OffsetPastEnd() >= l.lineOffset
	}
}

// nextLineIsAffectedBy returns true iff the given edit adds characters to,
// modifies, or deletes characters from the line following the line that was
// most recently read.
func (l *lineRdr) nextLineIsAffectedBy(e *edit) bool {
	if e == nil {
		return false
	} else if l.err == io.EOF {
		return false
	} else {
		return e.OffsetPastEnd() > l.offsetPastEnd()
	}
}

// startHunk creats a new hunk, adding the current line and up to numCtxLines
// lines of leading context.
func startHunk(lr *lineRdr) *hunk {
	h := &hunk{
		startOffset: lr.lineOffset,
		startLine:   lr.lineNum,
		numLines:    1,
	}

	for _, line := range lr.leadingCtxLines {
		h.startOffset -= len(line)
		h.startLine--
		h.numLines++
		h.hunk.WriteString(line)
	}

	h.hunk.WriteString(lr.line)
	return h
}

// editIter is an iterator for []edit slices.
type editIter struct {
	edits     []edit
	nextIndex int
}

// newEditIter creates a new editIter with the first edit in the given file
// marked.
func (e *EditSet) newEditIter() *editIter {
	return &editIter{e.edits, 0}
}

// edit returns the edit currently under the mark, or nil if no edits remain.
func (e *editIter) edit() *edit {
	if e.nextIndex >= len(e.edits) {
		return nil
	}
	return &e.edits[e.nextIndex]
}

// moveToNextEdit moves the mark to the next edit, and returns that edit (or
// nil if no edits remain).
func (e *editIter) moveToNextEdit() *edit {
	e.nextIndex++
	return e.edit()
}

// createPatch creates a Patch from an EditSet.  (The CreatePatch method on
// EditSet delegates to this function.)
func createPatch(e *EditSet, in io.Reader) (result *Patch, err error) {
	result = &Patch{}

	if len(e.edits) == 0 {
		return
	}

	reader := newLineRdr(in) // Reads lines from the original file
	it := e.newEditIter()    // Traverses edits (in order)
	var hunk *hunk           // Current hunk being added to
	var trailingCtxLines int // Number of unchanged lines at end of hunk

	// Iterate through each line, adding lines to a hunk if they are
	// affected by an edit or at most 2*numCtxLines following an edit;
	// add edits to the hunk whenever the last offset affected by that edit
	// is on the current line
	for err = reader.readLine(); err == nil || err == io.EOF; err = reader.readLine() {
		if hunk == nil {
			// No hunk has been started, so start one as soon as
			// we find a line that is changed
			if reader.currentLineIsAffectedBy(it.edit()) {
				hunk = startHunk(reader)
				last := addEditsOnCurLine(hunk, reader, it)
				if !reader.nextLineIsAffectedBy(last) {
					if reader.editAddsToStart(last) {
						trailingCtxLines = 1
					} else {
						trailingCtxLines = 0
					}
				}
			}
		} else {
			// A hunk has been started; add the current line, and
			// terminate the hunk after the maximum number of
			// trailing context lines have been added
			hunk.addLine(reader.line)
			if reader.currentLineIsAffectedBy(it.edit()) {
				last := addEditsOnCurLine(hunk, reader, it)
				if !reader.nextLineIsAffectedBy(last) {
					if reader.editAddsToStart(last) {
						trailingCtxLines = 1
					} else {
						trailingCtxLines = 0
					}
				}
			} else {
				trailingCtxLines++
				if trailingCtxLines > 2*numCtxLines {
					result.add(hunk)
					hunk = nil
				}
			}
		}
		if err == io.EOF {
			break
		}
	}
	if hunk != nil {
		result.add(hunk)
	}
	err = nil
	return
}

// addEditsOnCurLine begins with the current edit marked by the iterator it and
// adds that edit to the hunk as well as all subsequent edits whose last
// affected offset is on the current line.  It returns the last edit added to
// the hunk, or if no edits were added, the current edit marked by the
// iterator.
func addEditsOnCurLine(hunk *hunk, reader *lineRdr, it *editIter) *edit {
	lastEdit := it.edit()
	curEdit := lastEdit
	for reader.currentLineIsAffectedBy(curEdit) {
		if reader.nextLineIsAffectedBy(curEdit) {
			return lastEdit
		}
		lastEdit = curEdit
		hunk.addEdit(curEdit)
		curEdit = it.moveToNextEdit()
	}
	return lastEdit
}
