package rule

import (
	"fmt"
	"go/ast"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// IdenticalIfElseIfConditionsRule warns on if...else if chains with identical conditions.
type IdenticalIfElseIfConditionsRule struct{}

// Apply applies the rule to given file.
func (*IdenticalIfElseIfConditionsRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	getStmtLine := func(s ast.Stmt) int {
		return file.ToPosition(s.Pos()).Line
	}

	w := &rootWalkerIfElseIfIdenticalConditions{getStmtLine: getStmtLine, onFailure: onFailure}
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
func (*IdenticalIfElseIfConditionsRule) Name() string {
	return "identical-ifelseif-conditions"
}

type rootWalkerIfElseIfIdenticalConditions struct {
	getStmtLine func(ast.Stmt) int
	onFailure   func(lint.Failure)
}

func (w *rootWalkerIfElseIfIdenticalConditions) Visit(node ast.Node) ast.Visitor {
	n, ok := node.(*ast.IfStmt)
	if !ok {
		return w
	}

	_, isIfElseIf := n.Else.(*ast.IfStmt)
	if isIfElseIf {
		walker := &lintIfChainIdenticalConditions{
			onFailure:   w.onFailure,
			getStmtLine: w.getStmtLine,
			rootWalker:  w,
		}

		ast.Walk(walker, n)
		return nil // the walker already analyzed inner branches
	}

	return w
}

// walkBranch analyzes the given branch.
func (w *rootWalkerIfElseIfIdenticalConditions) walkBranch(branch ast.Stmt) {
	if branch == nil {
		return
	}

	walker := &rootWalkerIfElseIfIdenticalConditions{
		onFailure:   w.onFailure,
		getStmtLine: w.getStmtLine,
	}

	ast.Walk(walker, branch)
}

type lintIfChainIdenticalConditions struct {
	getStmtLine func(ast.Stmt) int
	onFailure   func(lint.Failure)
	conditions  map[string]int                         // condition hash -> line of the condition
	rootWalker  *rootWalkerIfElseIfIdenticalConditions // the walker to use to recursively analyze inner branches
}

// addCondition adds a condition to the set of if...else if conditions.
// If the set already contains the same condition it returns the line number of the identical condition.
func (w *lintIfChainIdenticalConditions) addCondition(condition ast.Expr, conditionLine int) (line int, match bool) {
	if condition == nil {
		return 0, false
	}

	if w.conditions == nil {
		w.resetConditions()
	}

	hash := astutils.NodeHash(condition)
	identical, ok := w.conditions[hash]
	if ok {
		return identical, true
	}

	w.conditions[hash] = conditionLine
	return 0, false
}

func (w *lintIfChainIdenticalConditions) resetConditions() {
	w.conditions = map[string]int{}
}

func (w *lintIfChainIdenticalConditions) Visit(node ast.Node) ast.Visitor {
	n, ok := node.(*ast.IfStmt)
	if !ok {
		return w
	}

	// recursively analyze the then-branch
	w.rootWalker.walkBranch(n.Body)

	if n.Init == nil { // only check if without initialization to avoid false positives
		currentCondLine := w.rootWalker.getStmtLine(n)
		identicalCondLine, match := w.addCondition(n.Cond, currentCondLine)
		if match {
			w.onFailure(lint.Failure{
				Confidence: 1.0,
				Node:       n,
				Category:   lint.FailureCategoryLogic,
				Failure:    fmt.Sprintf(`"if...else if" chain with identical conditions (lines %d and %d)`, identicalCondLine, currentCondLine),
			})
		}
	}

	if n.Else != nil {
		if chainedIf, ok := n.Else.(*ast.IfStmt); ok {
			w.Visit(chainedIf)
		} else {
			w.rootWalker.walkBranch(n.Else)
		}
	}

	w.resetConditions()
	return nil
}
