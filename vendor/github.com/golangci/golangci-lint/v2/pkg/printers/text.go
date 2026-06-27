package printers

import (
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

// Text prints issues with a human friendly representation.
type Text struct {
	printLinterName bool
	printIssuedLine bool
	colors          bool

	log logutils.Log
	w   io.Writer
}

func NewText(log logutils.Log, w io.Writer, cfg *config.Text) *Text {
	return &Text{
		printLinterName: cfg.PrintLinterName,
		printIssuedLine: cfg.PrintIssuedLine,
		colors:          cfg.Colors,
		log:             log.Child(logutils.DebugKeyTextPrinter),
		w:               w,
	}
}

func (p *Text) SprintfColored(ca color.Attribute, format string, args ...any) string {
	c := color.New(ca)

	if !p.colors {
		c.DisableColor()
	}

	return c.Sprintf(format, args...)
}

func (p *Text) Print(issues []*result.Issue) error {
	for _, issue := range issues {
		p.printIssue(issue)

		if !p.printIssuedLine {
			continue
		}

		p.printSourceCode(issue)
		p.printUnderLinePointer(issue)
	}

	return nil
}

func (p *Text) printIssue(issue *result.Issue) {
	text := p.SprintfColored(color.FgRed, "%s", strings.TrimSpace(issue.Text))
	if p.printLinterName {
		text += fmt.Sprintf(" (%s)", issue.FromLinter)
	}
	pos := p.SprintfColored(color.Bold, "%s:%d", issue.FilePath(), issue.Line())
	if issue.Pos.Column != 0 {
		pos += fmt.Sprintf(":%d", issue.Pos.Column)
	}
	fmt.Fprintf(p.w, "%s: %s\n", pos, text)
}

func (p *Text) printSourceCode(issue *result.Issue) {
	for _, line := range issue.SourceLines {
		fmt.Fprintln(p.w, line)
	}
}

func (p *Text) printUnderLinePointer(issue *result.Issue) {
	// if column == 0 it means column is unknown (e.g. for gosec)
	if len(issue.SourceLines) != 1 || issue.Pos.Column == 0 {
		return
	}

	col0 := issue.Pos.Column - 1
	line := issue.SourceLines[0]
	prefixRunes := make([]rune, 0, len(line))
	for j := 0; j < len(line) && j < col0; j++ {
		if line[j] == '\t' {
			prefixRunes = append(prefixRunes, '\t')
		} else {
			prefixRunes = append(prefixRunes, ' ')
		}
	}

	fmt.Fprintf(p.w, "%s%s\n", string(prefixRunes), p.SprintfColored(color.FgYellow, "^"))
}
