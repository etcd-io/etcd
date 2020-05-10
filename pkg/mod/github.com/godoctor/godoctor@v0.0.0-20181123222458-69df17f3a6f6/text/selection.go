// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines types representing a selection in a text editor, i.e.,
// a range of text within a file.

package text

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const fileNotFoundFmt = "The file %s was not found or was not loaded"

// A Selection represents a range of text within a particular file.  It is
// used to represent a selection in a text editor.
type Selection interface {
	// Convert returns start and end positions corresponding to this
	// selection.  It returns an error if this selection corresponds to a
	// file that is not in the given FileSet, or if the selected region is
	// not in range.
	Convert(*token.FileSet) (token.Pos, token.Pos, error)
	// GetFilename returns the file containing this selection.  The
	// returned filename may be an absolute or relative path and does is
	// not guaranteed to correspond to a valid file.
	GetFilename() string
	// String returns a human-readable representation of this Selection.
	String() string
}

// A LineColSelection is a Selection consisting of a filename, the line/column
// where the selected text begins, and the line/column where the text selection
// ends.  The end line and column must be greater than or equal to the start
// line and column, respectively.  Line and column numbers are 1-based.  The
// end position is inclusive, so a LineColSelection always represents a
// selection of at least one character.
type LineColSelection struct {
	Filename  string
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
}

// Convert returns start and end positions corresponding to this selection.  It
// returns an error if this selection corresponds to a file that is not in the
// given FileSet, or if the selected region is not in range.
func (lc *LineColSelection) Convert(fset *token.FileSet) (token.Pos, token.Pos, error) {
	file := findFile(fset, lc.Filename)
	if file == nil {
		return 0, 0, fmt.Errorf(fileNotFoundFmt, lc.Filename)
	}

	startPos, err := lineColToPos(file, lc.StartLine, lc.StartCol)
	if err != nil {
		return 0, 0, err
	}

	lastPos, err := lineColToPos(file, lc.EndLine, lc.EndCol)
	if err != nil {
		return 0, 0, err
	}

	// The end position should be exclusive, so set it to the offset of the
	// next character the end.  We may need to move more than one byte
	// ahead if the last character is non-ASCII.
	// (Unfortunately, the column information in Positions seems to be
	// incorrect for UTF-8 characters, but this should work if that's
	// fixed eventually...)
	endPos := lastPos + 1
	for file.Position(endPos).Column == file.Position(lastPos).Column {
		endPos++
	}
	if endPos < startPos {
		return 0, 0, fmt.Errorf("Invalid selection (end < start)")
	}

	if !startPos.IsValid() || !endPos.IsValid() {
		return 0, 0, fmt.Errorf("Invalid selection %v", lc)
	}
	return startPos, endPos, nil
}

// GetFilename returns the file containing this selection.  The returned
// filename may be an absolute or relative path and does is not guaranteed to
// correspond to a valid file.
func (lc *LineColSelection) GetFilename() string {
	return lc.Filename
}

func (lc *LineColSelection) String() string {
	return fmt.Sprintf("%s: %d,%d:%d,%d", lc.Filename,
		lc.StartLine, lc.StartCol, lc.EndLine, lc.EndCol)
}

// An OffsetLengthSelection is a Selection consisting of a filename, an offset
// (a nonnegative byte offset where the text selection begins), and a length (a
// nonnegative integer describing the number of bytes in the selection).  The
// first byte in a file is considered to be at offset 0.  A selection of length
// 0 is permissible.
type OffsetLengthSelection struct {
	Filename string
	Offset   int
	Length   int
}

// Convert returns start and end positions corresponding to this selection.  It
// returns an error if this selection corresponds to a file that is not in the
// given FileSet, or if the selected region is not in range.
func (ol *OffsetLengthSelection) Convert(fset *token.FileSet) (token.Pos, token.Pos, error) {
	file := findFile(fset, ol.Filename)
	if file == nil {
		return 0, 0, fmt.Errorf(fileNotFoundFmt, ol.Filename)
	}
	if ol.Offset < 0 || ol.Offset >= file.Size() {
		return 0, 0, fmt.Errorf("Invalid offset %d", ol.Offset)
	}
	if ol.Length < 0 || ol.Offset+ol.Length > file.Size() {
		return 0, 0, fmt.Errorf("Invalid length %d", ol.Length)
	}
	start := file.Pos(ol.Offset)
	end := file.Pos(ol.Offset + ol.Length)
	if !start.IsValid() || !end.IsValid() {
		return 0, 0, fmt.Errorf("Invalid selection %v", ol)
	}
	return start, end, nil
}

// GetFilename returns the file containing this selection.  The returned
// filename may be an absolute or relative path and does is not guaranteed to
// correspond to a valid file.
func (ol *OffsetLengthSelection) GetFilename() string {
	return ol.Filename
}

func (ol *OffsetLengthSelection) String() string {
	return fmt.Sprintf("%s: %d,%d", ol.Filename,
		ol.Offset, ol.Length)
}

// findFile returns the file corresponding to the given filename, or nil if no
// file can be found with that filename.  The absolute path of the returned
// file can be found via f.Name().
func findFile(fset *token.FileSet, filename string) *token.File {
	// from findQueryPos in go.tools/oracle/pos.go
	var file *token.File
	fset.Iterate(func(f *token.File) bool {
		if sameFile(filename, f.Name()) {
			file = f
			return false // done
		}
		return true // continue
	})
	return file
}

// sameFile returns true if target and check have the same basename and denote
// the same file.
// FIXME(jeff): Need to use filesystem here?
func sameFile(target, check string) bool { // from go.tools/oracle/pos.go
	if filepath.Base(target) == filepath.Base(check) { // (optimisation)
		if targetf, err := os.Stat(target); err == nil {
			if checkf, err := os.Stat(check); err == nil {
				return os.SameFile(targetf, checkf)
			}
		} else {
			// If the target filename does not exist (e.g., if it's
			// the "bogus" standard input file -.go or comes from
			// a FileSet populated with test data), fall back to
			// string comparison
			return target == check
		}
	}
	return false
}

// lineColToPos converts a line/column position to a token.Pos.  The first
// character in a file is considered to be at line 1, column 1.
func lineColToPos(file *token.File, line int, column int) (token.Pos, error) {
	if line < 1 || column < 1 {
		return token.NoPos, fmt.Errorf("Invalid position: line %d, column %d (line and column must be ≥ 1)", line, column)
	} else if line > file.LineCount() {
		return token.NoPos, fmt.Errorf("Invalid position: line %d, column %d (file contains %d lines)", line, column, file.LineCount())
	}

	// Binary search to find a position on the given line
	lastOffset := file.Size() - 1
	start := 0
	end := lastOffset
	mid := (start + end) / 2
	for start <= end {
		midLine := file.Line(file.Pos(mid))
		if line == midLine {
			break
		} else if line < midLine {
			end = mid - 1
		} else /* line > midLine */ {
			start = mid + 1
		}
		mid = (start + end) / 2
	}

	// Now mid is some position on the correct line; add/subtract to find
	// the position at the correct column
	difference := file.Position(file.Pos(mid)).Column - column
	pos := file.Pos(mid - difference)

	// The difference may have been underestimated if the line contains
	// non-ASCII characters
	for file.Position(pos).Column > column && pos > file.Pos(0) &&
		file.Position(pos-1).Column >= column {
		pos--
	}
	lastPos := file.Pos(file.Size() - 1)
	for file.Position(pos).Column < column && pos < lastPos &&
		file.Position(pos).Column < column {
		pos++
	}

	p := file.Position(pos)
	if p.Line != line || p.Column != column {
		return pos, fmt.Errorf("Invalid position: line %d, column %d (could only find line %d, column %d)",
			line, column, p.Line, p.Column)
	}
	return pos, nil
}

// NewSelection takes an input string of the form "line,col:line,col" or
// "offset,length" and returns a Selection (either LineColSelection or
// OffsetLengthSelection) corresponding to that selection in the given file.
func NewSelection(filename string, pos string) (Selection, error) {
	if ok, _ := regexp.MatchString("^\\d+,\\d+:\\d+,\\d+$", pos); ok {
		args := strings.Split(pos, ":")
		sl, sc := parseLineCol(args[0])
		el, ec := parseLineCol(args[1])
		if sl < 1 || sc < 1 || el < 1 || ec < 1 {
			return nil, fmt.Errorf("Line/column cannot be 0")
		}
		return &LineColSelection{
			Filename:  filename,
			StartLine: sl,
			StartCol:  sc,
			EndLine:   el,
			EndCol:    ec}, nil
	} else if ok, _ := regexp.MatchString("^\\d+,\\d+$", pos); ok {
		offset, length := parseLineCol(pos)
		if offset < 0 || length < 0 {
			return nil, fmt.Errorf("Invalid offset/length")
		}
		return &OffsetLengthSelection{
			Filename: filename,
			Offset:   offset,
			Length:   length}, nil
	}
	return nil, fmt.Errorf("invalid -pos %s", pos)
}

// parseLineCol parses a string consisting of two nonnegative integers (e.g.,
// "302,6") and returns the two integer values, or returns (-1,-1) if the
// input string does not have the correct format
func parseLineCol(linecol string) (int, int) {
	lc := strings.Split(linecol, ",")
	if l, err := strconv.ParseInt(lc[0], 10, 32); err == nil {
		if c, err := strconv.ParseInt(lc[1], 10, 32); err == nil {
			return int(l), int(c)
		}
	}
	return -1, -1
}
