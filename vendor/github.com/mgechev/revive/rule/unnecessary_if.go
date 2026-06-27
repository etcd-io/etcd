package rule

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// UnnecessaryIfRule warns on if...else statements that can be replaced by simpler expressions.
type UnnecessaryIfRule struct{}

// Apply applies the rule to given file.
func (*UnnecessaryIfRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	w := &lintUnnecessaryIf{onFailure: onFailure}
	for _, decl := range file.AST.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		ast.Walk(w, fn.Body)
	}

	return failures
}

// Name returns the rule name.
func (*UnnecessaryIfRule) Name() string {
	return "unnecessary-if"
}

type lintUnnecessaryIf struct {
	onFailure func(lint.Failure)
}

// Visit walks the AST looking for if statements of the form:
//
// if cond { return <bool literal> } else { return <bool literal> }
//
// or
//
// if cond { <idX> = <bool literal> } else { <idX> = <bool literal> }.
func (w *lintUnnecessaryIf) Visit(node ast.Node) ast.Visitor {
	ifStmt, ok := node.(*ast.IfStmt)
	if !ok {
		return w // not an if statement
	}

	//          if without else    or if with initialization
	mustSkip := ifStmt.Else == nil || ifStmt.Init != nil
	if mustSkip {
		return w
	}

	elseBranch, ok := ifStmt.Else.(*ast.BlockStmt)
	if !ok { // if-else-if construction
		return w // the rule only copes with single if...else statements
	}

	thenStmts := ifStmt.Body.List
	elseStmts := elseBranch.List
	if len(thenStmts) != 1 || len(elseStmts) != 1 {
		return w // then and else branches do not have just one statement
	}

	var replacement string
	var thenBool bool
	switch thenStmt := thenStmts[0].(type) {
	case *ast.ReturnStmt:
		replacement, thenBool = w.replacementForReturnStmt(thenStmt, elseStmts)
	case *ast.AssignStmt:
		replacement, thenBool = w.replacementForAssignmentStmt(thenStmt, elseStmts)
	default:
		return w // the then branch is neither a return nor an assignment
	}

	if replacement == "" {
		return w // no replacement found
	}

	cond := w.condAsString(ifStmt.Cond, !thenBool)
	msg := "replace this conditional by: " + replacement + " " + cond

	w.onFailure(lint.Failure{
		Confidence: 1.0,
		Node:       ifStmt,
		Category:   lint.FailureCategoryLogic,
		Failure:    msg,
	})

	return nil
}

var relationalOppositeOf = map[token.Token]token.Token{
	token.EQL: token.NEQ, // == !=
	token.GEQ: token.LSS, // >= <
	token.GTR: token.LEQ, // > <=
	token.LEQ: token.GTR, // <= >
	token.LSS: token.GEQ, // < >=
	token.NEQ: token.EQL, // != ==
}

// condAsString yields the string representation of the given condition expression.
// The method will try to minimize the negations in the resulting expression.
func (*lintUnnecessaryIf) condAsString(cond ast.Expr, mustNegate bool) string {
	result := astutils.GoFmt(cond)

	if mustNegate {
		result = "!(" + result + ")" // naive negation

		// check if we can build a simpler expression
		if binExp, ok := cond.(*ast.BinaryExpr); ok {
			originalOp := binExp.Op
			opposite, ok := relationalOppositeOf[originalOp]
			if ok {
				binExp.Op = opposite
				result = astutils.GoFmt(binExp) // replace initial result by a simpler one
				binExp.Op = originalOp
			}
		}
	}

	return result
}

// replacementForAssignmentStmt returns a replacement statement != ""
// iff both then and else statements are of the form <idX> = <bool literal>
// If the replacement != "" then the second return value is the Boolean value
// of <bool literal> in the then-side assignment, false otherwise.
func (w *lintUnnecessaryIf) replacementForAssignmentStmt(thenStmt *ast.AssignStmt, elseStmts []ast.Stmt) (replacement string, thenBool bool) {
	thenBoolStr, ok := w.isSingleBooleanLiteral(thenStmt.Rhs)
	if !ok {
		return "", false
	}

	thenLHS := astutils.GoFmt(thenStmt.Lhs[0])

	elseStmt, ok := elseStmts[0].(*ast.AssignStmt)
	if !ok {
		return "", false
	}

	elseLHS := astutils.GoFmt(elseStmt.Lhs[0])
	if thenLHS != elseLHS {
		return "", false
	}

	_, ok = w.isSingleBooleanLiteral(elseStmt.Rhs)
	if !ok {
		return "", false
	}

	return fmt.Sprintf("%s %s", thenLHS, thenStmt.Tok.String()), thenBoolStr == "true"
}

// replacementForReturnStmt returns a replacement statement != ""
// iff both then and else statements are of the form return <bool literal>
// If the replacement != "" then the second return value is the string representation
// of <bool literal> in the then-side assignment, "" otherwise.
func (w *lintUnnecessaryIf) replacementForReturnStmt(thenStmt *ast.ReturnStmt, elseStmts []ast.Stmt) (replacement string, thenBool bool) {
	thenBoolStr, ok := w.isSingleBooleanLiteral(thenStmt.Results)
	if !ok {
		return "", false
	}

	elseStmt, ok := elseStmts[0].(*ast.ReturnStmt)
	if !ok {
		return "", false
	}

	_, ok = w.isSingleBooleanLiteral(elseStmt.Results)
	if !ok {
		return "", false
	}

	return "return", thenBoolStr == "true"
}

// isSingleBooleanLiteral returns the string representation of <bool literal> and true
// if the given list of expressions has exactly one element and that element is a bool literal (true or false),
// otherwise it returns "" and false.
func (*lintUnnecessaryIf) isSingleBooleanLiteral(exprs []ast.Expr) (string, bool) {
	if len(exprs) != 1 {
		return "", false
	}

	ident, ok := exprs[0].(*ast.Ident)
	if !ok {
		return "", false
	}

	return ident.Name, (ident.Name == "true" || ident.Name == "false")
}
