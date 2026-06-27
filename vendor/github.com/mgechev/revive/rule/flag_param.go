package rule

import (
	"fmt"
	"go/ast"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// FlagParamRule warns on boolean parameters that create a control coupling.
type FlagParamRule struct{}

// Apply applies the rule to given file.
func (*FlagParamRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure
	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	for _, decl := range file.AST.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		isFuncWithNonEmptyBody := ok && fd.Body != nil
		if !isFuncWithNonEmptyBody {
			continue
		}

		boolParams := map[string]struct{}{}
		for _, param := range fd.Type.Params.List {
			if !astutils.IsIdent(param.Type, "bool") {
				continue
			}

			for _, paramIdent := range param.Names {
				boolParams[paramIdent.Name] = struct{}{}
			}
		}

		if len(boolParams) == 0 {
			continue
		}

		cv := conditionVisitor{boolParams, fd, onFailure}
		ast.Walk(cv, fd.Body)
	}

	return failures
}

// Name returns the rule name.
func (*FlagParamRule) Name() string {
	return "flag-parameter"
}

type conditionVisitor struct {
	idents    map[string]struct{}
	fd        *ast.FuncDecl
	onFailure func(lint.Failure)
}

func (w conditionVisitor) Visit(node ast.Node) ast.Visitor {
	ifStmt, ok := node.(*ast.IfStmt)
	if !ok {
		return w
	}

	findUsesOfIdents := func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return false
		}

		_, ok = w.idents[ident.Name]
		if !ok {
			return false
		}

		return w.idents[ident.Name] == struct{}{}
	}

	uses := astutils.SeekNode[*ast.Ident](ifStmt.Cond, findUsesOfIdents)
	if uses == nil {
		return w
	}

	w.onFailure(lint.Failure{
		Confidence: 1,
		Node:       w.fd.Type.Params,
		Category:   lint.FailureCategoryBadPractice,
		Failure:    fmt.Sprintf("parameter '%s' seems to be a control flag, avoid control coupling", uses.Name),
	})

	return nil
}
