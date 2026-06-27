// Package lint provides abstractions on top of go/analysis.
// These abstractions add extra information to analyzes, such as structured documentation and severities.
package lint

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
	"honnef.co/go/tools/analysis/facts/tokenfile"
)

// Analyzer wraps a go/analysis.Analyzer and provides structured documentation.
type Analyzer struct {
	// The analyzer's documentation. Unlike go/analysis.Analyzer.Doc,
	// this field is structured, providing access to severity, options
	// etc.
	Doc      *RawDocumentation
	Analyzer *analysis.Analyzer
}

func InitializeAnalyzer(a *Analyzer) *Analyzer {
	a.Analyzer.Doc = a.Doc.Compile().String()
	a.Analyzer.URL = "https://staticcheck.dev/docs/checks/#" + a.Analyzer.Name
	a.Analyzer.Requires = append(a.Analyzer.Requires, tokenfile.Analyzer)
	return a
}

// Severity describes the severity of diagnostics reported by an analyzer.
type Severity int

const (
	SeverityNone Severity = iota
	SeverityError
	SeverityDeprecated
	SeverityWarning
	SeverityInfo
	SeverityHint
)

// MergeStrategy sets how merge mode should behave for diagnostics of an analyzer.
type MergeStrategy int

const (
	MergeIfAny MergeStrategy = iota
	MergeIfAll
)

type RawDocumentation struct {
	Title      string
	Text       string
	Before     string
	After      string
	Since      string
	NonDefault bool
	Options    []string
	Severity   Severity
	MergeIf    MergeStrategy
}

type Documentation struct {
	Title string
	Text  string

	TitleMarkdown string
	TextMarkdown  string

	Before     string
	After      string
	Since      string
	NonDefault bool
	Options    []string
	Severity   Severity
	MergeIf    MergeStrategy
}

func (doc RawDocumentation) Compile() *Documentation {
	return &Documentation{
		Title: strings.TrimSpace(stripMarkdown(doc.Title)),
		Text:  strings.TrimSpace(stripMarkdown(doc.Text)),

		TitleMarkdown: strings.TrimSpace(toMarkdown(doc.Title)),
		TextMarkdown:  strings.TrimSpace(toMarkdown(doc.Text)),

		Before:     strings.TrimSpace(doc.Before),
		After:      strings.TrimSpace(doc.After),
		Since:      doc.Since,
		NonDefault: doc.NonDefault,
		Options:    doc.Options,
		Severity:   doc.Severity,
		MergeIf:    doc.MergeIf,
	}
}

func toMarkdown(s string) string {
	return strings.NewReplacer(`\'`, "`", `\"`, "`").Replace(s)
}

func stripMarkdown(s string) string {
	return strings.NewReplacer(`\'`, "", `\"`, "'").Replace(s)
}

func (doc *Documentation) Format(metadata bool) string {
	return doc.format(false, metadata)
}

func (doc *Documentation) FormatMarkdown(metadata bool) string {
	return doc.format(true, metadata)
}

func (doc *Documentation) format(markdown bool, metadata bool) string {
	b := &strings.Builder{}
	if markdown {
		fmt.Fprintf(b, "%s\n\n", doc.TitleMarkdown)
		if doc.Text != "" {
			fmt.Fprintf(b, "%s\n\n", doc.TextMarkdown)
		}
	} else {
		fmt.Fprintf(b, "%s\n\n", doc.Title)
		if doc.Text != "" {
			fmt.Fprintf(b, "%s\n\n", doc.Text)
		}
	}

	if doc.Before != "" {
		fmt.Fprintln(b, "Before:")
		fmt.Fprintln(b, "")
		for line := range strings.SplitSeq(doc.Before, "\n") {
			fmt.Fprint(b, "    ", line, "\n")
		}
		fmt.Fprintln(b, "")
		fmt.Fprintln(b, "After:")
		fmt.Fprintln(b, "")
		for line := range strings.SplitSeq(doc.After, "\n") {
			fmt.Fprint(b, "    ", line, "\n")
		}
		fmt.Fprintln(b, "")
	}

	if metadata {
		fmt.Fprint(b, "Available since\n    ")
		if doc.Since == "" {
			fmt.Fprint(b, "unreleased")
		} else {
			fmt.Fprintf(b, "%s", doc.Since)
		}
		if doc.NonDefault {
			fmt.Fprint(b, ", non-default")
		}
		fmt.Fprint(b, "\n")
		if len(doc.Options) > 0 {
			fmt.Fprintf(b, "\nOptions\n")
			for _, opt := range doc.Options {
				fmt.Fprintf(b, "    %s", opt)
			}
			fmt.Fprint(b, "\n")
		}
	}

	return b.String()
}

func (doc *Documentation) String() string {
	return doc.Format(true)
}

// ExhaustiveTypeSwitch panics when called. It can be used to ensure
// that type switches are exhaustive.
func ExhaustiveTypeSwitch(v any) {
	panic(fmt.Sprintf("internal error: unhandled case %T", v))
}

// A directive is a comment of the form '//lint:<command>
// [arguments...]'. It represents instructions to the static analysis
// tool.
type Directive struct {
	Command   string
	Arguments []string
	Directive *ast.Comment
	Node      ast.Node
}

func parseDirective(s string) (cmd string, args []string) {
	if !strings.HasPrefix(s, "//lint:") {
		return "", nil
	}
	s = strings.TrimPrefix(s, "//lint:")
	fields := strings.Split(s, " ")
	return fields[0], fields[1:]
}

// ParseDirectives extracts all directives from a list of Go files.
func ParseDirectives(files []*ast.File, fset *token.FileSet) []Directive {
	var dirs []Directive
	for _, f := range files {
		// OPT(dh): in our old code, we skip all the comment map work if we
		// couldn't find any directives, benchmark if that's actually
		// worth doing
		cm := ast.NewCommentMap(fset, f, f.Comments)
		for node, cgs := range cm {
			for _, cg := range cgs {
				for _, c := range cg.List {
					if !strings.HasPrefix(c.Text, "//lint:") {
						continue
					}
					cmd, args := parseDirective(c.Text)
					d := Directive{
						Command:   cmd,
						Arguments: args,
						Directive: c,
						Node:      node,
					}
					dirs = append(dirs, d)
				}
			}
		}
	}
	return dirs
}
