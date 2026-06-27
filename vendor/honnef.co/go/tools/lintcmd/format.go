package lintcmd

import (
	"encoding/json"
	"fmt"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"

	"honnef.co/go/tools/analysis/lint"
)

func shortPath(path string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return path
	}
	if rel, err := filepath.Rel(cwd, path); err == nil && len(rel) < len(path) {
		return rel
	}
	return path
}

func relativePositionString(pos token.Position) string {
	s := shortPath(pos.Filename)
	if pos.IsValid() {
		if s != "" {
			s += ":"
		}
		s += fmt.Sprintf("%d:%d", pos.Line, pos.Column)
	}
	if s == "" {
		s = "-"
	}
	return s
}

type statter interface {
	Stats(total, errors, warnings, ignored int)
}

type formatter interface {
	Format(checks []*lint.Analyzer, diagnostics []diagnostic)
}

type textFormatter struct {
	W io.Writer
}

func (o textFormatter) Format(_ []*lint.Analyzer, ps []diagnostic) {
	for _, p := range ps {
		fmt.Fprintf(o.W, "%s: %s\n", relativePositionString(p.Position), p.String())
		for _, r := range p.Related {
			fmt.Fprintf(o.W, "\t%s: %s\n", relativePositionString(r.Position), r.Message)
		}
	}
}

type nullFormatter struct{}

func (nullFormatter) Format([]*lint.Analyzer, []diagnostic) {}

type jsonFormatter struct {
	W io.Writer
}

func (o jsonFormatter) Format(_ []*lint.Analyzer, ps []diagnostic) {
	type location struct {
		File   string `json:"file"`
		Line   int    `json:"line"`
		Column int    `json:"column"`
	}
	type related struct {
		Location location `json:"location"`
		End      location `json:"end"`
		Message  string   `json:"message"`
	}

	enc := json.NewEncoder(o.W)
	for _, p := range ps {
		jp := struct {
			Code     string    `json:"code"`
			Severity string    `json:"severity,omitempty"`
			Location location  `json:"location"`
			End      location  `json:"end"`
			Message  string    `json:"message"`
			Related  []related `json:"related,omitempty"`
		}{
			Code:     p.Category,
			Severity: p.Severity.String(),
			Location: location{
				File:   p.Position.Filename,
				Line:   p.Position.Line,
				Column: p.Position.Column,
			},
			End: location{
				File:   p.End.Filename,
				Line:   p.End.Line,
				Column: p.End.Column,
			},
			Message: p.Message,
		}
		for _, r := range p.Related {
			jp.Related = append(jp.Related, related{
				Location: location{
					File:   r.Position.Filename,
					Line:   r.Position.Line,
					Column: r.Position.Column,
				},
				End: location{
					File:   r.End.Filename,
					Line:   r.End.Line,
					Column: r.End.Column,
				},
				Message: r.Message,
			})
		}
		_ = enc.Encode(jp)
	}
}

type stylishFormatter struct {
	W io.Writer

	prevFile string
	tw       *tabwriter.Writer
}

func (o *stylishFormatter) Format(_ []*lint.Analyzer, ps []diagnostic) {
	for _, p := range ps {
		pos := p.Position
		if pos.Filename == "" {
			pos.Filename = "-"
		}

		if pos.Filename != o.prevFile {
			if o.prevFile != "" {
				o.tw.Flush()
				fmt.Fprintln(o.W)
			}
			fmt.Fprintln(o.W, pos.Filename)
			o.prevFile = pos.Filename
			o.tw = tabwriter.NewWriter(o.W, 0, 4, 2, ' ', 0)
		}
		fmt.Fprintf(o.tw, "  (%d, %d)\t%s\t%s\n", pos.Line, pos.Column, p.Category, p.Message)
		for _, r := range p.Related {
			fmt.Fprintf(o.tw, "    (%d, %d)\t\t  %s\n", r.Position.Line, r.Position.Column, r.Message)
		}
	}
}

func (o *stylishFormatter) Stats(total, errors, warnings, ignored int) {
	if o.tw != nil {
		o.tw.Flush()
		fmt.Fprintln(o.W)
	}
	fmt.Fprintf(o.W, " ✖ %d problems (%d errors, %d warnings, %d ignored)\n",
		total, errors, warnings, ignored)
}
