package diff

import (
	"bytes"
	"errors"
	"fmt"
)

// ReverseFileDiff takes a diff.FileDiff, and returns the reverse operation.
// This is a FileDiff that undoes the edit of the original.
func ReverseFileDiff(fd *FileDiff) (*FileDiff, error) {
	reverse := FileDiff{
		OrigName: fd.NewName,
		OrigTime: fd.NewTime,
		NewName:  fd.OrigName,
		NewTime:  fd.OrigTime,
		Extended: fd.Extended,
	}
	for _, hunk := range fd.Hunks {
		invHunk, err := reverseHunk(hunk)
		if err != nil {
			return nil, err
		}
		reverse.Hunks = append(reverse.Hunks, invHunk)
	}
	return &reverse, nil
}

// ReverseMultiFileDiff reverses a series of FileDiffs.
func ReverseMultiFileDiff(fds []*FileDiff) ([]*FileDiff, error) {
	var reverse []*FileDiff
	for _, fd := range fds {
		r, err := ReverseFileDiff(fd)
		if err != nil {
			return nil, err
		}
		reverse = append(reverse, r)
	}
	return reverse, nil
}

// A subhunk represents a portion of a Hunk.Body, split into three sections.
// It consists of zero or more context lines, followed by zero or more orig
// lines and then zero or more new lines.
//
// Each line is stored WITHOUT its starting character, but with the newlines
// included.  The final entry in a section may be missing a trailing newline.
//
// A missing newline in orig is represented in a Hunk by OrigNoNewlineAt,
// but is represented here as a missing newline.
type contextLine struct {
	body []byte
	bare bool
}

type subhunk struct {
	context []contextLine
	orig    [][]byte
	new     [][]byte
}

// reverseHunk converts a Hunk into its reverse operation.
func reverseHunk(forward *Hunk) (*Hunk, error) {
	reverse := Hunk{
		OrigStartLine:   forward.NewStartLine,
		OrigLines:       forward.NewLines,
		OrigNoNewlineAt: 0, // we may change this below
		NewStartLine:    forward.OrigStartLine,
		NewLines:        forward.OrigLines,
		Section:         forward.Section,
		StartPosition:   forward.StartPosition,
	}
	subs, err := toSubhunks(forward)
	if err != nil {
		return nil, err
	}
	for _, sub := range subs {
		invSub := subhunk{
			context: sub.context,
			orig:    sub.new,
			new:     sub.orig,
		}
		for _, line := range invSub.context {
			if line.bare {
				reverse.Body = append(reverse.Body, line.body...)
				continue
			}
			reverse.Body = append(reverse.Body, ' ')
			reverse.Body = append(reverse.Body, line.body...)
		}
		for _, line := range invSub.orig {
			reverse.Body = append(reverse.Body, '-')
			reverse.Body = append(reverse.Body, line...)
		}
		if len(invSub.orig) > 0 && reverse.Body[len(reverse.Body)-1] != '\n' {
			// There was a missing newline in `orig`, which we encode in a
			// hunk with an offset.
			reverse.Body = append(reverse.Body, '\n')
			reverse.OrigNoNewlineAt = int32(len(reverse.Body))
		}
		for _, line := range invSub.new {
			reverse.Body = append(reverse.Body, '+')
			reverse.Body = append(reverse.Body, line...)
		}
	}
	return &reverse, nil
}

func extractContextLines(from *[]byte) []contextLine {
	var lines []contextLine
	for len(*from) > 0 {
		if (*from)[0] == '\n' {
			lines = append(lines, contextLine{body: []byte{'\n'}, bare: true})
			*from = (*from)[1:]
			continue
		}
		if (*from)[0] != ' ' {
			break
		}

		newline := bytes.IndexByte(*from, '\n')
		if newline < 0 {
			lines = append(lines, contextLine{body: (*from)[1:]})
			*from = nil
			continue
		}

		lines = append(lines, contextLine{body: (*from)[1 : newline+1]})
		*from = (*from)[newline+1:]
	}
	return lines
}

func extractLinesStartingWith(from *[]byte, startingWith byte) [][]byte {
	var lines [][]byte
	for len(*from) > 0 {
		if (*from)[0] != startingWith {
			break
		}

		newline := bytes.IndexByte(*from, '\n')
		if newline < 0 {
			lines = append(lines, (*from)[1:])
			*from = nil
			continue
		}

		lines = append(lines, (*from)[1:newline+1])
		*from = (*from)[newline+1:]
	}
	return lines
}

// Extracts the subhunks from a diff.Hunk.
//
// This groups a Hunk's buffer into one or more subhunks, matching the conditions
// of `subhunk` above.  This function groups, strips prefix characters, and strips
// a newline for `OrigNoNewlineAt` if necessary.
func toSubhunks(hunk *Hunk) ([]subhunk, error) {
	var body []byte = hunk.Body
	var subhunks []subhunk
	if len(body) == 0 {
		return nil, nil
	}
	for len(body) > 0 {
		sh := subhunk{
			context: extractContextLines(&body),
			orig:    extractLinesStartingWith(&body, '-'),
			new:     extractLinesStartingWith(&body, '+'),
		}
		if len(sh.context) == 0 && len(sh.orig) == 0 && len(sh.new) == 0 {
			// The first line didn't start with any expected prefix.
			return nil, fmt.Errorf("unexpected character %q at start of line", body[0])
		}
		subhunks = append(subhunks, sh)
	}
	if hunk.OrigNoNewlineAt > 0 {
		// The Hunk represents a missing newline at the end of an "orig" line with a
		// OrigNoNewlineAt index.  We represent it here as an actual missing newline.
		var lastSubhunk *subhunk = &subhunks[len(subhunks)-1]
		s := len(lastSubhunk.orig)
		if s == 0 {
			return nil, errors.New("inconsistent OrigNoNewlineAt in input")
		}
		var cut bool
		lastSubhunk.orig[s-1], cut = bytes.CutSuffix(lastSubhunk.orig[s-1], []byte("\n"))
		if !cut {
			return nil, errors.New("missing newline in input")
		}
	}
	return subhunks, nil
}
