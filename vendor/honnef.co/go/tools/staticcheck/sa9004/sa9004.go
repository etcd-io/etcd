package sa9004

import (
	"go/ast"
	"go/token"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ast/astutil"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA9004",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Only the first constant has an explicit type`,

		Text: `In a constant declaration such as the following:

    const (
        First byte = 1
        Second     = 2
    )

the constant Second does not have the same type as the constant First.
This construct shouldn't be confused with

    const (
        First byte = iota
        Second
    )

where \'First\' and \'Second\' do indeed have the same type. The type is only
passed on when no explicit value is assigned to the constant.

When declaring enumerations with explicit values it is therefore
important not to write

    const (
          EnumFirst EnumType = 1
          EnumSecond         = 2
          EnumThird          = 3
    )

This discrepancy in types can cause various confusing behaviors and
bugs.


Wrong type in variable declarations

The most obvious issue with such incorrect enumerations expresses
itself as a compile error:

    package pkg

    const (
        EnumFirst  uint8 = 1
        EnumSecond       = 2
    )

    func fn(useFirst bool) {
        x := EnumSecond
        if useFirst {
            x = EnumFirst
        }
    }

fails to compile with

    ./const.go:11:5: cannot use EnumFirst (type uint8) as type int in assignment


Losing method sets

A more subtle issue occurs with types that have methods and optional
interfaces. Consider the following:

    package main

    import "fmt"

    type Enum int

    func (e Enum) String() string {
        return "an enum"
    }

    const (
        EnumFirst  Enum = 1
        EnumSecond      = 2
    )

    func main() {
        fmt.Println(EnumFirst)
        fmt.Println(EnumSecond)
    }

This code will output

    an enum
    2

as \'EnumSecond\' has no explicit type, and thus defaults to \'int\'.`,
		Since:    "2019.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	fn := func(node ast.Node) {
		decl := node.(*ast.GenDecl)
		if !decl.Lparen.IsValid() {
			return
		}
		if decl.Tok != token.CONST {
			return
		}

		groups := astutil.GroupSpecs(pass.Fset, decl.Specs)
	groupLoop:
		for _, group := range groups {
			if len(group) < 2 {
				continue
			}
			if group[0].(*ast.ValueSpec).Type == nil {
				// first constant doesn't have a type
				continue groupLoop
			}

			firstType := pass.TypesInfo.TypeOf(group[0].(*ast.ValueSpec).Values[0])
			for i, spec := range group {
				spec := spec.(*ast.ValueSpec)
				if i > 0 && spec.Type != nil {
					continue groupLoop
				}
				if len(spec.Names) != 1 || len(spec.Values) != 1 {
					continue groupLoop
				}

				if !types.ConvertibleTo(pass.TypesInfo.TypeOf(spec.Values[0]), firstType) {
					continue groupLoop
				}

				switch v := spec.Values[0].(type) {
				case *ast.BasicLit:
				case *ast.UnaryExpr:
					if _, ok := v.X.(*ast.BasicLit); !ok {
						continue groupLoop
					}
				default:
					// if it's not a literal it might be typed, such as
					// time.Microsecond = 1000 * Nanosecond
					continue groupLoop
				}
			}
			var edits []analysis.TextEdit
			typ := group[0].(*ast.ValueSpec).Type
			for _, spec := range group[1:] {
				nspec := *spec.(*ast.ValueSpec)
				nspec.Type = typ
				// The position of `spec` node excludes comments (if any).
				// However, on generating the source back from the node, the comments are included. Setting `Comment` to nil ensures deduplication of comments.
				nspec.Comment = nil
				edits = append(edits, edit.ReplaceWithNode(pass.Fset, spec, &nspec))
			}
			report.Report(pass, group[0],
				"only the first constant in this group has an explicit type",
				report.Fixes(edit.Fix("Add type to all constants in group", edits...)))
		}
	}
	code.Preorder(pass, fn, (*ast.GenDecl)(nil))
	return nil, nil
}
