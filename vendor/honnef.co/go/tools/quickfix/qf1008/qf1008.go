package qf1008

import (
	"fmt"
	"go/ast"
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
		Name:     "QF1008",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:    "Omit embedded fields from selector expression",
		Since:    "2021.1",
		Severity: lint.SeverityHint,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	type Selector struct {
		Node   *ast.SelectorExpr
		X      ast.Expr
		Fields []*ast.Ident
	}

	// extractSelectors extracts uninterrupted sequences of selector expressions.
	// For example, for a.b.c().d.e[0].f.g three sequences will be returned: (X=a, X.b.c), (X=a.b.c(), X.d.e), and (X=a.b.c().d.e[0], X.f.g)
	//
	// It returns nil if the provided selector expression is not the root of a set of sequences.
	// For example, for a.b.c, if node is b.c, no selectors will be returned.
	extractSelectors := func(expr *ast.SelectorExpr) []Selector {
		path, _ := astutil.PathEnclosingInterval(code.File(pass, expr), expr.Pos(), expr.Pos())
		for i := len(path) - 1; i >= 0; i-- {
			if el, ok := path[i].(*ast.SelectorExpr); ok {
				if el != expr {
					// this expression is a subset of the entire chain, don't look at it.
					return nil
				}
				break
			}
		}

		inChain := false
		var out []Selector
		for _, el := range path {
			if expr, ok := el.(*ast.SelectorExpr); ok {
				if !inChain {
					inChain = true
					out = append(out, Selector{X: expr.X})
				}
				sel := &out[len(out)-1]
				sel.Fields = append(sel.Fields, expr.Sel)
				sel.Node = expr
			} else if inChain {
				inChain = false
			}
		}
		return out
	}

	fn := func(node ast.Node) {
		expr := node.(*ast.SelectorExpr)

		if _, ok := expr.X.(*ast.SelectorExpr); !ok {
			// Avoid the expensive call to PathEnclosingInterval for the common 1-level deep selector, which cannot be shortened.
			return
		}

		sels := extractSelectors(expr)
		if len(sels) == 0 {
			return
		}

		for _, sel := range sels {
		fieldLoop:
			for base, fields := pass.TypesInfo.TypeOf(sel.X), sel.Fields; len(fields) >= 2; base, fields = pass.TypesInfo.ObjectOf(fields[0]).Type(), fields[1:] {
				hop1 := fields[0]
				hop2 := fields[1]

				// the selector expression might be a qualified identifier, which cannot be simplified
				if base == types.Typ[types.Invalid] {
					continue fieldLoop
				}

				// Check if we can skip a field in the chain of selectors.
				// We can skip a field 'b' if a.b.c and a.c resolve to the same object and take the same path.
				//
				// We set addressable to true unconditionally because we've already successfully type-checked the program,
				// which means either the selector doesn't need addressability, or it is addressable.
				leftObj, leftLeg, _ := types.LookupFieldOrMethod(base, true, pass.Pkg, hop1.Name)

				// We can't skip fields that aren't embedded
				if !leftObj.(*types.Var).Embedded() {
					continue fieldLoop
				}

				directObj, directPath, _ := types.LookupFieldOrMethod(base, true, pass.Pkg, hop2.Name)

				// Fail fast if omitting the embedded field leads to a different object
				if directObj != pass.TypesInfo.ObjectOf(hop2) {
					continue fieldLoop
				}

				_, rightLeg, _ := types.LookupFieldOrMethod(leftObj.Type(), true, pass.Pkg, hop2.Name)

				// Fail fast if the paths are obviously different
				if len(directPath) != len(leftLeg)+len(rightLeg) {
					continue fieldLoop
				}

				// Make sure that omitting the embedded field will take the same path to the final object.
				// Multiple paths involving different fields may lead to the same type-checker object, causing different runtime behavior.
				for i := range directPath {
					if i < len(leftLeg) {
						if leftLeg[i] != directPath[i] {
							continue fieldLoop
						}
					} else {
						if rightLeg[i-len(leftLeg)] != directPath[i] {
							continue fieldLoop
						}
					}
				}

				e := edit.Delete(edit.Range{hop1.Pos(), hop2.Pos()})
				report.Report(pass, hop1, fmt.Sprintf("could remove embedded field %q from selector", hop1.Name),
					report.Fixes(edit.Fix(fmt.Sprintf("Remove embedded field %q from selector", hop1.Name), e)))
			}
		}
	}
	code.Preorder(pass, fn, (*ast.SelectorExpr)(nil))
	return nil, nil
}
