package noinlineerr

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const errMessage = "avoid inline error handling using `if err := ...; err != nil`; use plain assignment `err := ...`"

func NewAnalyzer() *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     "noinlineerr",
		Doc:      "Disallows inline error handling (`if err := ...; err != nil {`)",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}
}

func run(pass *analysis.Pass) (any, error) {
	insp, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, nil //nolint:nilnil // nothing to return
	}

	nodeFilter := []ast.Node{
		(*ast.IfStmt)(nil),
	}

	insp.Preorder(nodeFilter, inlineErrorInspector(pass))

	return nil, nil //nolint:nilnil // nothing to return
}

func inlineErrorInspector(pass *analysis.Pass) func(n ast.Node) {
	return func(n ast.Node) {
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok || ifStmt.Init == nil {
			return
		}

		// check if the init clause is an assignment
		assignStmt, ok := ifStmt.Init.(*ast.AssignStmt)
		if !ok {
			return
		}

		// iterate over left-hand side variables of the assignment
		for _, lhs := range assignStmt.Lhs {
			ident, ok := lhs.(*ast.Ident)
			if !ok {
				continue
			}

			// confirm type is error and it is used in condition
			obj := pass.TypesInfo.ObjectOf(ident)
			if !isError(obj) || ident.Name == "_" || !errorUsedInCondition(ifStmt.Cond, ident.Name) {
				continue
			}

			// if there are more than 1 assignment
			// or there are any variables with same name
			// then we can make a shadow conflict with other variables
			// so don't do anything beside simple error message
			if len(assignStmt.Lhs) != 1 || shadowVarsExists(ident.Name, pass.TypesInfo.Scopes[ifStmt]) {
				pass.Reportf(ident.Pos(), errMessage)
				return
			}

			// else we know there is a simple err assignment like
			// if err := func(); err != nil {}
			// and we can autofix that

			var buf bytes.Buffer

			_ = printer.Fprint(&buf, pass.Fset, assignStmt)
			assignText := buf.String()

			// report usage of inline error assignment
			pass.Report(analysis.Diagnostic{
				Pos:     ident.Pos(),
				End:     ident.End(),
				Message: errMessage,
				SuggestedFixes: []analysis.SuggestedFix{
					{
						Message: "move err assignment outside if",
						TextEdits: []analysis.TextEdit{
							{
								// insert err := ... before if
								Pos:     ifStmt.Pos(),
								End:     ifStmt.Pos(),
								NewText: []byte(assignText + "\n"),
							},
							{
								// delete Init part
								Pos:     assignStmt.Pos(),
								End:     assignStmt.End() + 1, // +1 for ;
								NewText: nil,
							},
						},
					},
				},
			})
		}
	}
}

func isError(obj types.Object) bool {
	if obj == nil {
		return false
	}

	errorType := types.Universe.Lookup("error").Type()

	return types.AssignableTo(obj.Type(), errorType)
}

func shadowVarsExists(name string, scope *types.Scope) bool {
	if scope == nil {
		return false
	}

	parentScope := scope.Parent()
	if parentScope == nil {
		return false
	}

	return parentScope.Lookup(name) != nil
}

func errorUsedInCondition(cond ast.Expr, errIdentName string) bool {
	used := false

	ast.Inspect(cond, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}

		if ident.Name == errIdentName {
			used = true
			return false
		}

		return true
	})

	return used
}
