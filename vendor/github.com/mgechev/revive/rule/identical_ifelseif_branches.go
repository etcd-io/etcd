package rule

import (
	"fmt"
	"go/ast"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// IdenticalIfElseIfBranchesRule warns on if...else if chains with identical branches.
type IdenticalIfElseIfBranchesRule struct{}

// Apply applies the rule to given file.
func (*IdenticalIfElseIfBranchesRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	getStmtLine := func(s ast.Stmt) int {
		return file.ToPosition(s.Pos()).Line
	}

	w := &rootWalkerIfElseIfIdenticalBranches{getStmtLine: getStmtLine, onFailure: onFailure}
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
func (*IdenticalIfElseIfBranchesRule) Name() string {
	return "identical-ifelseif-branches"
}

type rootWalkerIfElseIfIdenticalBranches struct {
	getStmtLine func(ast.Stmt) int
	onFailure   func(lint.Failure)
}

func (w *rootWalkerIfElseIfIdenticalBranches) Visit(node ast.Node) ast.Visitor {
	n, ok := node.(*ast.IfStmt)
	if !ok {
		return w
	}

	_, isIfElseIf := n.Else.(*ast.IfStmt)
	if isIfElseIf {
		walker := &lintIfChainIdenticalBranches{
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
func (w *rootWalkerIfElseIfIdenticalBranches) walkBranch(branch ast.Stmt) {
	if branch == nil {
		return
	}

	walker := &rootWalkerIfElseIfIdenticalBranches{
		onFailure:   w.onFailure,
		getStmtLine: w.getStmtLine,
	}

	ast.Walk(walker, branch)
}

type lintIfChainIdenticalBranches struct {
	getStmtLine         func(ast.Stmt) int
	onFailure           func(lint.Failure)
	branches            []ast.Stmt                           // hold branches to compare
	rootWalker          *rootWalkerIfElseIfIdenticalBranches // the walker to use to recursively analyze inner branches
	hasComplexCondition bool                                 // indicates if one of the if conditions is "complex"
}

// addBranch adds a branch to the list of branches to be compared.
func (w *lintIfChainIdenticalBranches) addBranch(branch ast.Stmt) {
	if branch == nil {
		return
	}

	if w.branches == nil {
		w.resetBranches()
	}

	w.branches = append(w.branches, branch)
}

// resetBranches resets (clears) the list of branches to compare.
func (w *lintIfChainIdenticalBranches) resetBranches() {
	w.branches = []ast.Stmt{}
	w.hasComplexCondition = false
}

func (w *lintIfChainIdenticalBranches) Visit(node ast.Node) ast.Visitor {
	n, ok := node.(*ast.IfStmt)
	if !ok {
		return w
	}

	// recursively analyze the then-branch
	w.rootWalker.walkBranch(n.Body)

	if n.Init == nil { // only check if without initialization to avoid false positives
		w.addBranch(n.Body)
	}

	if w.isComplexCondition(n.Cond) {
		w.hasComplexCondition = true
	}

	if n.Else != nil {
		if chainedIf, ok := n.Else.(*ast.IfStmt); ok {
			w.Visit(chainedIf)
		} else {
			w.addBranch(n.Else)
			w.rootWalker.walkBranch(n.Else)
		}
	}

	identicalBranches := w.identicalBranches(w.branches)
	for _, branchPair := range identicalBranches {
		msg := fmt.Sprintf(`"if...else if" chain with identical branches (lines %d and %d)`, branchPair[0], branchPair[1])
		confidence := 1.0
		if w.hasComplexCondition {
			confidence = 0.8
		}
		w.onFailure(lint.Failure{
			Confidence: confidence,
			Node:       w.branches[0],
			Category:   lint.FailureCategoryLogic,
			Failure:    msg,
		})
	}

	w.resetBranches()
	return nil
}

// isComplexCondition returns true if the given expression is "complex", false otherwise.
// An expression is considered complex if it has a function call.
func (*lintIfChainIdenticalBranches) isComplexCondition(expr ast.Expr) bool {
	call := astutils.SeekNode[*ast.CallExpr](expr, func(n ast.Node) bool {
		_, ok := n.(*ast.CallExpr)
		return ok
	})

	return call != nil
}

// identicalBranches yields pairs of (line numbers) of identical branches from the given branches.
func (w *lintIfChainIdenticalBranches) identicalBranches(branches []ast.Stmt) [][]int {
	result := [][]int{}
	if len(branches) < 2 {
		return result // no other branch to compare thus we return
	}

	hashes := map[string]int{} // branch code hash -> branch line
	for _, branch := range branches {
		hash := astutils.NodeHash(branch)
		branchLine := w.getStmtLine(branch)
		if match, ok := hashes[hash]; ok {
			result = append(result, []int{match, branchLine})
		}

		hashes[hash] = branchLine
	}

	return result
}
