package rule

import (
	"fmt"
	"go/ast"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// ModifiesParamRule warns on assignments to function parameters.
type ModifiesParamRule struct{}

// modifyingParamPositions are parameter positions that are modified by a function.
type modifyingParamPositions = []int

// modifyingFunctions maps function names to the positions of parameters they modify.
var modifyingFunctions = map[string]modifyingParamPositions{
	"slices.Delete":     {0},
	"slices.DeleteFunc": {0},
}

// Apply applies the rule to given file.
func (*ModifiesParamRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	w := lintModifiesParamRule{onFailure: onFailure}
	ast.Walk(&w, file.AST)
	return failures
}

// Name returns the rule name.
func (*ModifiesParamRule) Name() string {
	return "modifies-parameter"
}

type lintModifiesParamRule struct {
	params    map[string]bool
	onFailure func(lint.Failure)
}

func retrieveParamNames(pl []*ast.Field) map[string]bool {
	result := make(map[string]bool, len(pl))
	for _, p := range pl {
		for _, n := range p.Names {
			if n.Name == "_" {
				continue
			}

			result[n.Name] = true
		}
	}
	return result
}

func (w *lintModifiesParamRule) Visit(node ast.Node) ast.Visitor {
	switch v := node.(type) {
	case *ast.FuncDecl:
		w.params = retrieveParamNames(v.Type.Params.List)
	case *ast.IncDecStmt:
		if id, ok := v.X.(*ast.Ident); ok {
			checkParam(id, w)
		}
	case *ast.AssignStmt:
		lhs := v.Lhs
		for i, e := range lhs {
			id, ok := e.(*ast.Ident)
			if !ok {
				continue
			}

			if i < len(v.Rhs) {
				w.checkModifyingFunction(v.Rhs[i])
			}
			checkParam(id, w)
		}
	case *ast.ExprStmt:
		w.checkModifyingFunction(v.X)
	}

	return w
}

func checkParam(id *ast.Ident, w *lintModifiesParamRule) {
	if w.params[id.Name] {
		w.onFailure(lint.Failure{
			Confidence: 0.5, // confidence is low because of shadow variables
			Node:       id,
			Category:   lint.FailureCategoryBadPractice,
			Failure:    fmt.Sprintf("parameter '%s' seems to be modified", id),
		})
	}
}

func (w *lintModifiesParamRule) checkModifyingFunction(callNode ast.Node) {
	callExpr, ok := callNode.(*ast.CallExpr)
	if !ok {
		return
	}

	funcName := astutils.GoFmt(callExpr.Fun)
	positions, found := modifyingFunctions[funcName]
	if !found {
		return
	}

	for _, pos := range positions {
		if pos >= len(callExpr.Args) {
			return
		}

		id, ok := callExpr.Args[pos].(*ast.Ident)
		if !ok {
			continue
		}

		_, match := w.params[id.Name]
		if !match {
			continue
		}

		w.onFailure(lint.Failure{
			Confidence: 0.5, // confidence is low because of shadow variables
			Node:       callExpr,
			Category:   lint.FailureCategoryBadPractice,
			Failure:    fmt.Sprintf("parameter '%s' seems to be modified by '%s'", id.Name, funcName),
		})
	}
}
