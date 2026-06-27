package checker

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"path"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Type of violation: can be method or function
type ViolationType int

const (
	Function ViolationType = iota + 1
	Method
)

const (
	Strings     string = "string"
	Bytes       string = "[]byte"
	Byte        string = "byte"
	Rune        string = "rune"
	UntypedRune string = "untyped rune"
)

// Violation describes what message we going to give to a particular code violation
type Violation struct {
	Type     ViolationType //
	Args     []int         // Indexes of the arguments needs to be checked
	ArgsType string

	Targets    string
	Package    string
	AltPackage string
	Struct     string
	Caller     string
	AltCaller  string

	// --- tests generation information
	Generate *Generate

	// --- suggestions related info about violation of rules.
	base      []byte           // receiver of the method or pkg name
	callExpr  *ast.CallExpr    // actual call expression, to extract arguments
	arguments map[int]ast.Expr // fixed arguments
}

// Tests (generation) related struct.
type Generate struct {
	SkipGenerate bool
	PreCondition string   // Precondition we want to be generated
	Pattern      string   // Generate pattern (for the `want` message)
	Returns      []string // ReturnTypes as slice
}

func (v *Violation) With(base []byte, e *ast.CallExpr, args map[int]ast.Expr) *Violation {
	v.base = base
	v.callExpr = e
	v.arguments = args

	return v
}

func (v *Violation) getArgType() string {
	if v.ArgsType != "" {
		return v.ArgsType
	}

	if v.Targets == Strings {
		return Bytes
	}

	return Strings
}

func (v *Violation) Message() string {
	if v.Type == Method {
		return fmt.Sprintf("avoid allocations with (*%s.%s).%s",
			path.Base(v.Package), v.Struct, v.AltCaller)
	}

	pkg := v.Package
	if len(v.AltPackage) > 0 {
		pkg = v.AltPackage
	}

	return fmt.Sprintf("avoid allocations with %s.%s", path.Base(pkg), v.AltCaller)
}

func (v *Violation) suggest(fSet *token.FileSet) []byte {
	var buf bytes.Buffer

	if len(v.base) > 0 {
		buf.Write(v.base)
		buf.WriteString(".")
	}

	buf.WriteString(v.AltCaller)
	buf.WriteByte('(')
	for idx := range v.callExpr.Args {
		if arg, ok := v.arguments[idx]; ok {
			printer.Fprint(&buf, fSet, arg)
		} else {
			printer.Fprint(&buf, fSet, v.callExpr.Args[idx])
		}

		if idx != len(v.callExpr.Args)-1 {
			buf.WriteString(", ")
		}
	}
	buf.WriteByte(')')

	return buf.Bytes()
}

func (v *Violation) Diagnostic(fSet *token.FileSet) analysis.Diagnostic {
	diagnostic := analysis.Diagnostic{
		Pos:     v.callExpr.Pos(),
		End:     v.callExpr.Pos(),
		Message: v.Message(),
	}

	var buf bytes.Buffer
	printer.Fprint(&buf, fSet, v.callExpr)
	noNl := bytes.IndexByte(buf.Bytes(), '\n') < 0

	// Struct based fix.
	if v.Type == Method && noNl {
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Fix Issue With",
			TextEdits: []analysis.TextEdit{{
				Pos: v.callExpr.Pos(), End: v.callExpr.End(), NewText: v.suggest(fSet),
			}},
		}}
	}

	if v.AltPackage == "" {
		v.AltPackage = v.Package
	}

	// Hooray! we don't need to change package and redo imports.
	if v.Type == Function && v.AltPackage == v.Package && noNl {
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Fix Issue With",
			TextEdits: []analysis.TextEdit{{
				Pos: v.callExpr.Pos(), End: v.callExpr.End(), NewText: v.suggest(fSet),
			}},
		}}
	}

	// do not change

	return diagnostic
}

type GolangIssue struct {
	Start     token.Position
	End       token.Position
	Message   string
	InlineFix string
	Original  string
}

// Issue intended to be used only within `golangci-lint`, but you can use it
// alongside Diagnostic if you wish.
func (v *Violation) Issue(fSet *token.FileSet) GolangIssue {
	issue := GolangIssue{
		Start:   fSet.Position(v.callExpr.Pos()),
		End:     fSet.Position(v.callExpr.End()),
		Message: v.Message(),
	}

	// original expression (useful for debug & required for replace)
	var buf bytes.Buffer
	printer.Fprint(&buf, fSet, v.callExpr)
	issue.Original = buf.String()

	noNl := strings.IndexByte(issue.Original, '\n') < 0

	if v.Type == Method && noNl {
		fix := v.suggest(fSet)
		issue.InlineFix = string(fix)
	}

	if v.AltPackage == "" {
		v.AltPackage = v.Package
	}

	// Hooray! we don't need to change package and redo imports.
	if v.Type == Function && v.AltPackage == v.Package && noNl {
		fix := v.suggest(fSet)
		issue.InlineFix = string(fix)
	}

	return issue
}

// ofType normalize input types (mostly typed and untyped runes).
func normalType(s string) string {
	if s == UntypedRune {
		return Rune
	}
	return s
}
