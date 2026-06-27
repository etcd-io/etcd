package internal

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"

	diffpkg "github.com/sourcegraph/go-diff/diff"
	"golang.org/x/tools/go/analysis"

	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
)

type Change struct {
	From, To int
	NewLines []string
}

type diffLineType string

const (
	diffLineAdded    diffLineType = "added"
	diffLineOriginal diffLineType = "original"
	diffLineDeleted  diffLineType = "deleted"
)

type diffLine struct {
	originalNumber int // 1-based original line number
	typ            diffLineType
	data           string // "+" or "-" stripped line
}

type hunkChangesParser struct {
	// needed because we merge currently added lines with the last original line
	lastOriginalLine *diffLine

	// if the first line of diff is an adding we save all additions to replacementLinesToPrepend
	replacementLinesToPrepend []string

	log logutils.Log

	changes []Change
}

func (p *hunkChangesParser) parse(h *diffpkg.Hunk) []Change {
	lines := parseDiffLines(h)

	for i := 0; i < len(lines); {
		line := lines[i]

		if line.typ == diffLineOriginal {
			p.handleOriginalLine(lines, line, &i)
			continue
		}

		var deletedLines []diffLine
		for ; i < len(lines) && lines[i].typ == diffLineDeleted; i++ {
			deletedLines = append(deletedLines, lines[i])
		}

		var addedLines []string
		for ; i < len(lines) && lines[i].typ == diffLineAdded; i++ {
			addedLines = append(addedLines, lines[i].data)
		}

		if len(deletedLines) != 0 {
			p.handleDeletedLines(deletedLines, addedLines)
			continue
		}

		// no deletions, only additions
		p.handleAddedOnlyLines(addedLines)
	}

	if len(p.replacementLinesToPrepend) != 0 {
		p.log.Infof("The diff contains only additions: no original or deleted lines: %#v", lines)
		return nil
	}

	return p.changes
}

func (p *hunkChangesParser) handleOriginalLine(lines []diffLine, line diffLine, i *int) {
	if len(p.replacementLinesToPrepend) == 0 {
		p.lastOriginalLine = &line
		*i++
		return
	}

	// check following added lines for the case:
	// + added line 1
	// original line
	// + added line 2

	*i++
	var followingAddedLines []string
	for ; *i < len(lines) && lines[*i].typ == diffLineAdded; *i++ {
		followingAddedLines = append(followingAddedLines, lines[*i].data)
	}

	change := Change{
		From:     line.originalNumber,
		To:       line.originalNumber,
		NewLines: slices.Concat(p.replacementLinesToPrepend, []string{line.data}, followingAddedLines),
	}
	p.changes = append(p.changes, change)

	p.replacementLinesToPrepend = nil
	p.lastOriginalLine = &line
}

func (p *hunkChangesParser) handleDeletedLines(deletedLines []diffLine, addedLines []string) {
	change := Change{
		From: deletedLines[0].originalNumber,
		To:   deletedLines[len(deletedLines)-1].originalNumber,
	}

	switch {
	case len(addedLines) != 0:
		change.NewLines = slices.Concat(p.replacementLinesToPrepend, addedLines)
		p.replacementLinesToPrepend = nil

	case len(p.replacementLinesToPrepend) != 0:
		// delete-only change with possible prepending
		change.NewLines = slices.Clone(p.replacementLinesToPrepend)
		p.replacementLinesToPrepend = nil
	}

	p.changes = append(p.changes, change)
}

func (p *hunkChangesParser) handleAddedOnlyLines(addedLines []string) {
	if p.lastOriginalLine == nil {
		// the first line is added; the diff looks like:
		// 1. + ...
		// 2. - ...
		// or
		// 1. + ...
		// 2. ...

		p.replacementLinesToPrepend = addedLines

		return
	}

	// add-only change merged into the last original line with possible prepending
	change := Change{
		From:     p.lastOriginalLine.originalNumber,
		To:       p.lastOriginalLine.originalNumber,
		NewLines: slices.Concat(p.replacementLinesToPrepend, []string{p.lastOriginalLine.data}, addedLines),
	}

	p.changes = append(p.changes, change)

	p.replacementLinesToPrepend = nil
}

func parseDiffLines(h *diffpkg.Hunk) []diffLine {
	lines := bytes.Split(h.Body, []byte{'\n'})

	currentOriginalLineNumber := int(h.OrigStartLine)

	var diffLines []diffLine

	for i, line := range lines {
		dl := diffLine{
			originalNumber: currentOriginalLineNumber,
		}

		if i == len(lines)-1 && len(line) == 0 {
			// handle last \n: don't add an empty original line
			break
		}

		lineStr := string(line)

		switch {
		case strings.HasPrefix(lineStr, "-"):
			dl.typ = diffLineDeleted
			dl.data = strings.TrimPrefix(lineStr, "-")
			currentOriginalLineNumber++

		case strings.HasPrefix(lineStr, "+"):
			dl.typ = diffLineAdded
			dl.data = strings.TrimPrefix(lineStr, "+")

		default:
			dl.typ = diffLineOriginal
			dl.data = strings.TrimPrefix(lineStr, " ")
			currentOriginalLineNumber++
		}

		diffLines = append(diffLines, dl)
	}

	// if > 0, then the original file had a 'No newline at end of file' mark
	if h.OrigNoNewlineAt > 0 {
		dl := diffLine{
			originalNumber: currentOriginalLineNumber + 1,
			typ:            diffLineAdded,
			data:           "",
		}
		diffLines = append(diffLines, dl)
	}

	return diffLines
}

func ExtractDiagnosticFromPatch(
	pass *analysis.Pass,
	file *ast.File,
	patch []byte,
	logger logutils.Log,
) error {
	diffs, err := diffpkg.ParseMultiFileDiff(patch)
	if err != nil {
		return fmt.Errorf("can't parse patch: %w", err)
	}

	if len(diffs) == 0 {
		return fmt.Errorf("got no diffs from patch parser: %s", patch)
	}

	ft := pass.Fset.File(file.Pos())

	adjLine := pass.Fset.PositionFor(file.Pos(), false).Line - pass.Fset.PositionFor(file.Pos(), true).Line

	for _, d := range diffs {
		if len(d.Hunks) == 0 {
			logger.Warnf("Got no hunks in diff %+v", d)
			continue
		}

		for _, hunk := range d.Hunks {
			p := hunkChangesParser{log: logger}

			changes := p.parse(hunk)

			for _, change := range changes {
				pass.Report(toDiagnostic(ft, change, adjLine))
			}
		}
	}

	return nil
}

func toDiagnostic(ft *token.File, change Change, adjLine int) analysis.Diagnostic {
	from := min(change.From+adjLine, ft.LineCount())

	start := ft.LineStart(from)

	end := goanalysis.EndOfLinePos(ft, change.To+adjLine)

	return analysis.Diagnostic{
		Pos:     start,
		End:     end,
		Message: "File is not properly formatted",
		SuggestedFixes: []analysis.SuggestedFix{{
			TextEdits: []analysis.TextEdit{{
				Pos:     start,
				End:     end,
				NewText: []byte(strings.Join(change.NewLines, "\n")),
			}},
		}},
	}
}
