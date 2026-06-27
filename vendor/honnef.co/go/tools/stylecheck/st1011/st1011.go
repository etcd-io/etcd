package st1011

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/types/typeutil"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "ST1011",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Poorly chosen name for variable of type \'time.Duration\'`,
		Text: `\'time.Duration\' values represent an amount of time, which is represented
as a count of nanoseconds. An expression like \'5 * time.Microsecond\'
yields the value \'5000\'. It is therefore not appropriate to suffix a
variable of type \'time.Duration\' with any time unit, such as \'Msec\' or
\'Milli\'.`,
		Since:   `2019.1`,
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	suffixes := []string{
		"Sec", "Secs", "Seconds",
		"Msec", "Msecs",
		"Milli", "Millis", "Milliseconds",
		"Usec", "Usecs", "Microseconds",
		"MS", "Ms",
	}
	fn := func(names []*ast.Ident) {
		for _, name := range names {
			if _, ok := pass.TypesInfo.Defs[name]; !ok {
				continue
			}
			T := pass.TypesInfo.TypeOf(name)
			if !typeutil.IsTypeWithName(T, "time.Duration") && !typeutil.IsPointerToTypeWithName(T, "time.Duration") {
				continue
			}
			for _, suffix := range suffixes {
				if strings.HasSuffix(name.Name, suffix) {
					report.Report(pass, name, fmt.Sprintf("var %s is of type %v; don't use unit-specific suffix %q", name.Name, T, suffix))
					break
				}
			}
		}
	}

	fn2 := func(node ast.Node) {
		switch node := node.(type) {
		case *ast.ValueSpec:
			fn(node.Names)
		case *ast.FieldList:
			for _, field := range node.List {
				fn(field.Names)
			}
		case *ast.AssignStmt:
			if node.Tok != token.DEFINE {
				break
			}
			var names []*ast.Ident
			for _, lhs := range node.Lhs {
				if lhs, ok := lhs.(*ast.Ident); ok {
					names = append(names, lhs)
				}
			}
			fn(names)
		}
	}

	code.Preorder(pass, fn2, (*ast.ValueSpec)(nil), (*ast.FieldList)(nil), (*ast.AssignStmt)(nil))
	return nil, nil
}
