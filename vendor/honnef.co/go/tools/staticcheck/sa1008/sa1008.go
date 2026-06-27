package sa1008

import (
	"fmt"
	"go/ast"
	"net/http"
	"strconv"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1008",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Non-canonical key in \'http.Header\' map`,
		Text: `Keys in \'http.Header\' maps are canonical, meaning they follow a specific
combination of uppercase and lowercase letters. Methods such as
\'http.Header.Add\' and \'http.Header.Del\' convert inputs into this canonical
form before manipulating the map.

When manipulating \'http.Header\' maps directly, as opposed to using the
provided methods, care should be taken to stick to canonical form in
order to avoid inconsistencies. The following piece of code
demonstrates one such inconsistency:

    h := http.Header{}
    h["etag"] = []string{"1234"}
    h.Add("etag", "5678")
    fmt.Println(h)

    // Output:
    // map[Etag:[5678] etag:[1234]]

The easiest way of obtaining the canonical form of a key is to use
\'http.CanonicalHeaderKey\'.`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	fn := func(node ast.Node, push bool) bool {
		if !push {
			return false
		}
		if assign, ok := node.(*ast.AssignStmt); ok {
			// TODO(dh): This risks missing some Header reads, for
			// example in `h1["foo"] = h2["foo"]` – these edge
			// cases are probably rare enough to ignore for now.
			for _, expr := range assign.Lhs {
				op, ok := expr.(*ast.IndexExpr)
				if !ok {
					continue
				}
				if code.IsOfTypeWithName(pass, op.X, "net/http.Header") {
					return false
				}
			}
			return true
		}
		op, ok := node.(*ast.IndexExpr)
		if !ok {
			return true
		}
		if !code.IsOfTypeWithName(pass, op.X, "net/http.Header") {
			return true
		}
		s, ok := code.ExprToString(pass, op.Index)
		if !ok {
			return true
		}
		canonical := http.CanonicalHeaderKey(s)
		if s == canonical {
			return true
		}
		var fix analysis.SuggestedFix
		switch op.Index.(type) {
		case *ast.BasicLit:
			fix = edit.Fix("Canonicalize header key", edit.ReplaceWithString(op.Index, strconv.Quote(canonical)))
		case *ast.Ident:
			call := &ast.CallExpr{
				Fun:  edit.Selector("http", "CanonicalHeaderKey"),
				Args: []ast.Expr{op.Index},
			}
			fix = edit.Fix("Wrap in http.CanonicalHeaderKey", edit.ReplaceWithNode(pass.Fset, op.Index, call))
		}
		msg := fmt.Sprintf("keys in http.Header are canonicalized, %q is not canonical; fix the constant or use http.CanonicalHeaderKey", s)
		if fix.Message != "" {
			report.Report(pass, op, msg, report.Fixes(fix))
		} else {
			report.Report(pass, op, msg)
		}
		return true
	}
	pass.ResultOf[inspect.Analyzer].(*inspector.Inspector).Nodes([]ast.Node{(*ast.AssignStmt)(nil), (*ast.IndexExpr)(nil)}, fn)
	return nil, nil
}
