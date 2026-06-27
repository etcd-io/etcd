package copyloopvar

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var checkAlias bool

func NewAnalyzer() *analysis.Analyzer {
	analyzer := &analysis.Analyzer{
		Name: "copyloopvar",
		Doc:  "a linter detects places where loop variables are copied",
		Run:  run,
		Requires: []*analysis.Analyzer{
			inspect.Analyzer,
		},
	}
	analyzer.Flags.BoolVar(&checkAlias, "check-alias", false, "check all assigning the loop variable to another variable")
	return analyzer
}

func run(pass *analysis.Pass) (any, error) {
	pass.ResultOf[inspect.Analyzer].(*inspector.Inspector).Preorder([]ast.Node{
		(*ast.RangeStmt)(nil),
		(*ast.ForStmt)(nil),
	}, func(n ast.Node) {
		switch node := n.(type) {
		case *ast.RangeStmt:
			checkRangeStmt(pass, node)
		case *ast.ForStmt:
			checkForStmt(pass, node)
		}
	})

	return nil, nil
}

func checkRangeStmt(pass *analysis.Pass, rangeStmt *ast.RangeStmt) {
	key, ok := rangeStmt.Key.(*ast.Ident)
	if !ok {
		return
	}
	var value *ast.Ident
	if rangeStmt.Value != nil {
		if value, ok = rangeStmt.Value.(*ast.Ident); !ok {
			return
		}
	}
	for _, stmt := range rangeStmt.Body.List {
		assignStmt, ok := stmt.(*ast.AssignStmt)
		if !ok {
			continue
		}
		if assignStmt.Tok != token.DEFINE {
			continue
		}
		for i, rh := range assignStmt.Rhs {
			right, ok := rh.(*ast.Ident)
			if !ok {
				continue
			}
			if right.Name != key.Name && (value == nil || right.Name != value.Name) {
				continue
			}
			if !checkAlias {
				left, ok := assignStmt.Lhs[i].(*ast.Ident)
				if !ok {
					continue
				}
				if left.Name != right.Name {
					continue
				}
			}

			report(pass, assignStmt, right, i)
		}
	}
}

func checkForStmt(pass *analysis.Pass, forStmt *ast.ForStmt) {
	if forStmt.Init == nil {
		return
	}
	initAssignStmt, ok := forStmt.Init.(*ast.AssignStmt)
	if !ok {
		return
	}
	initVarNameMap := make(map[string]interface{}, len(initAssignStmt.Lhs))
	for _, lh := range initAssignStmt.Lhs {
		if initVar, ok := lh.(*ast.Ident); ok {
			initVarNameMap[initVar.Name] = struct{}{}
		}
	}
	for _, stmt := range forStmt.Body.List {
		assignStmt, ok := stmt.(*ast.AssignStmt)
		if !ok {
			continue
		}
		if assignStmt.Tok != token.DEFINE {
			continue
		}
		for i, rh := range assignStmt.Rhs {
			right, ok := rh.(*ast.Ident)
			if !ok {
				continue
			}
			if _, ok := initVarNameMap[right.Name]; !ok {
				continue
			}
			if !checkAlias {
				left, ok := assignStmt.Lhs[i].(*ast.Ident)
				if !ok {
					continue
				}
				if left.Name != right.Name {
					continue
				}
			}

			report(pass, assignStmt, right, i)
		}
	}
}

func report(pass *analysis.Pass, assignStmt *ast.AssignStmt, right *ast.Ident, i int) {
	diagnostic := analysis.Diagnostic{
		Pos:     assignStmt.Pos(),
		Message: fmt.Sprintf(`The copy of the 'for' variable "%s" can be deleted (Go 1.22+)`, right.Name),
	}

	if i == 0 && isSimpleAssignStmt(assignStmt, right) {
		diagnostic.SuggestedFixes = append(diagnostic.SuggestedFixes, analysis.SuggestedFix{
			TextEdits: []analysis.TextEdit{{
				Pos:     assignStmt.Pos(),
				End:     assignStmt.End(),
				NewText: nil,
			}},
		})
	}

	pass.Report(diagnostic)
}

func isSimpleAssignStmt(assignStmt *ast.AssignStmt, rhs *ast.Ident) bool {
	if len(assignStmt.Lhs) != 1 {
		return false
	}

	lhs, ok := assignStmt.Lhs[0].(*ast.Ident)
	if !ok {
		return false
	}

	return rhs.Name == lhs.Name
}
