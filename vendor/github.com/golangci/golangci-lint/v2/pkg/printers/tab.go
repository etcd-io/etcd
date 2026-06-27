package printers

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/fatih/color"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

// Tab prints issues using tabulation as a field separator.
type Tab struct {
	printLinterName bool
	colors          bool

	log logutils.Log
	w   io.Writer
}

func NewTab(log logutils.Log, w io.Writer, cfg *config.Tab) *Tab {
	return &Tab{
		printLinterName: cfg.PrintLinterName,
		colors:          cfg.Colors,
		log:             log.Child(logutils.DebugKeyTabPrinter),
		w:               w,
	}
}

func (p *Tab) SprintfColored(ca color.Attribute, format string, args ...any) string {
	c := color.New(ca)

	if !p.colors {
		c.DisableColor()
	}

	return c.Sprintf(format, args...)
}

func (p *Tab) Print(issues []*result.Issue) error {
	w := tabwriter.NewWriter(p.w, 0, 0, 2, ' ', 0)

	for _, issue := range issues {
		p.printIssue(issue, w)
	}

	if err := w.Flush(); err != nil {
		p.log.Warnf("Can't flush tab writer: %s", err)
	}

	return nil
}

func (p *Tab) printIssue(issue *result.Issue, w io.Writer) {
	text := p.SprintfColored(color.FgRed, "%s", issue.Text)
	if p.printLinterName {
		text = fmt.Sprintf("%s\t%s", issue.FromLinter, text)
	}

	pos := p.SprintfColored(color.Bold, "%s:%d", issue.FilePath(), issue.Line())
	if issue.Pos.Column != 0 {
		pos += fmt.Sprintf(":%d", issue.Pos.Column)
	}

	fmt.Fprintf(w, "%s\t%s\n", pos, text)
}
