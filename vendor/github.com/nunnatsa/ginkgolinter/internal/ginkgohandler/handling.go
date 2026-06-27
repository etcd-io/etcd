package ginkgohandler

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"

	"github.com/nunnatsa/ginkgolinter/config"
)

const (
	linterName            = "ginkgo-linter"
	focusContainerFound   = linterName + ": Focus container found. This is used only for local debug and should not be part of the actual source code. Consider to replace with %q"
	focusSpecFound        = linterName + ": Focus spec found. This is used only for local debug and should not be part of the actual source code. Consider to remove it"
	useBeforeEachTemplate = "use BeforeEach() to assign variable %s"
)

func handleGinkgoSpecs(expr ast.Expr, config config.Config, pass *analysis.Pass, ginkgoHndlr Handler) bool {
	goDeeper := false
	if exp, ok := expr.(*ast.CallExpr); ok {
		if config.ForbidFocus && checkFocusContainer(pass, ginkgoHndlr, exp) {
			goDeeper = true
		}

		if config.ForbidSpecPollution && checkAssignmentsInContainer(pass, ginkgoHndlr, exp) {
			goDeeper = true
		}
	}
	return goDeeper
}

func checkAssignmentsInContainer(pass *analysis.Pass, ginkgoHndlr Handler, exp *ast.CallExpr) bool {
	foundSomething := false
	if ginkgoHndlr.isWrapContainer(exp) {
		for _, arg := range exp.Args {
			if fn, ok := arg.(*ast.FuncLit); ok {
				if fn.Body != nil {
					if checkAssignments(pass, fn.Body.List) {
						foundSomething = true
					}
					break
				}
			}
		}
	}

	return foundSomething
}

func checkAssignments(pass *analysis.Pass, list []ast.Stmt) bool {
	foundSomething := false
	for _, stmt := range list {
		switch st := stmt.(type) {
		case *ast.DeclStmt:
			if checkAssignmentDecl(pass, st) {
				foundSomething = true
			}

		case *ast.AssignStmt:
			if checkAssignmentAssign(pass, st) {
				foundSomething = true
			}

		case *ast.IfStmt:
			if checkAssignmentIf(pass, st) {
				foundSomething = true
			}
		}
	}

	return foundSomething
}

func checkAssignmentsValues(pass *analysis.Pass, names []*ast.Ident, values []ast.Expr) bool {
	foundSomething := false
	for i, val := range values {
		if !is[*ast.FuncLit](val) {
			reportNoFix(pass, names[i].Pos(), useBeforeEachTemplate, names[i].Name)
			foundSomething = true
		}
	}

	return foundSomething
}

func checkAssignmentDecl(pass *analysis.Pass, ds *ast.DeclStmt) bool {
	foundSomething := false
	if gen, ok := ds.Decl.(*ast.GenDecl); ok {
		if gen.Tok != token.VAR {
			return false
		}
		for _, spec := range gen.Specs {
			if valSpec, ok := spec.(*ast.ValueSpec); ok {
				if checkAssignmentsValues(pass, valSpec.Names, valSpec.Values) {
					foundSomething = true
				}
			}
		}
	}

	return foundSomething
}

func checkAssignmentAssign(pass *analysis.Pass, as *ast.AssignStmt) bool {
	foundSomething := false
	for i, val := range as.Rhs {
		if !is[*ast.FuncLit](val) {
			if id, isIdent := as.Lhs[i].(*ast.Ident); isIdent && id.Name != "_" {
				reportNoFix(pass, id.Pos(), useBeforeEachTemplate, id.Name)
				foundSomething = true
			}
		}
	}
	return foundSomething
}

func checkAssignmentIf(pass *analysis.Pass, is *ast.IfStmt) bool {
	foundSomething := false

	if is.Body != nil {
		if checkAssignments(pass, is.Body.List) {
			foundSomething = true
		}
	}
	if is.Else != nil {
		if block, isBlock := is.Else.(*ast.BlockStmt); isBlock {
			if checkAssignments(pass, block.List) {
				foundSomething = true
			}
		}
	}

	return foundSomething
}

func checkFocusContainer(pass *analysis.Pass, handler Handler, exp *ast.CallExpr) bool {
	foundFocus := false
	isFocus, id := handler.getFocusContainerName(exp)
	if isFocus {
		reportNewName(pass, id, id.Name[1:], id.Name)
		foundFocus = true
	}

	if id != nil && isContainer(id.Name) {
		for _, arg := range exp.Args {
			if handler.isFocusSpec(arg) {
				reportNoFix(pass, arg.Pos(), focusSpecFound)
				foundFocus = true
			} else if callExp, ok := arg.(*ast.CallExpr); ok {
				if checkFocusContainer(pass, handler, callExp) { // handle table entries
					foundFocus = true
				}
			}
		}
	}

	return foundFocus
}

func reportNewName(pass *analysis.Pass, id *ast.Ident, newName string, oldExpr string) {
	pass.Report(analysis.Diagnostic{
		Pos:     id.Pos(),
		Message: fmt.Sprintf(focusContainerFound, newName),
		SuggestedFixes: []analysis.SuggestedFix{
			{
				Message: fmt.Sprintf("should replace %s with %s", oldExpr, newName),
				TextEdits: []analysis.TextEdit{
					{
						Pos:     id.Pos(),
						End:     id.End(),
						NewText: []byte(newName),
					},
				},
			},
		},
	})
}

func reportNoFix(pass *analysis.Pass, pos token.Pos, message string, args ...any) {
	if len(args) > 0 {
		message = fmt.Sprintf(message, args...)
	}

	pass.Report(analysis.Diagnostic{
		Pos:     pos,
		Message: message,
	})
}

func is[T any](x any) bool {
	_, matchType := x.(T)
	return matchType
}
