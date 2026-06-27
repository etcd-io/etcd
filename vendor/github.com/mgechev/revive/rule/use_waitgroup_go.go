package rule

import (
	"go/ast"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// UseWaitGroupGoRule spots Go idioms that might be rewritten using [sync.WaitGroup.Go].
type UseWaitGroupGoRule struct{}

// Apply applies the rule to given file.
func (*UseWaitGroupGoRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	if !file.Pkg.IsAtLeastGoVersion(lint.Go125) {
		return nil // skip analysis if Go version < 1.25
	}

	var failures []lint.Failure

	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	w := &lintUseWaitGroupGo{
		onFailure: onFailure,
	}

	// Iterate over declarations looking for function declarations
	for _, decl := range file.AST.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue // not a function
		}

		if fn.Body == nil {
			continue // external (no-Go) function
		}

		// Analyze the function body
		ast.Walk(w, fn.Body)
	}

	return failures
}

// Name returns the rule name.
func (*UseWaitGroupGoRule) Name() string {
	return "use-waitgroup-go"
}

type lintUseWaitGroupGo struct {
	onFailure func(lint.Failure)
}

func (w *lintUseWaitGroupGo) Visit(node ast.Node) ast.Visitor {
	// Only interested in blocks of statements
	block, ok := node.(*ast.BlockStmt)
	if !ok {
		return w // not a block of statements
	}

	w.analyzeBlock(block)

	return w
}

// analyzeBlock searches AST subtrees with the following form
// wg.Add(...)
// ...
//
//	go func (...) {
//	   ...
//	   wg.Done // or defer wg.Done
//	   ...
//	}
//
// Warning: the analysis only looks for exactly wg.Add and wg.Done, that means
// calls to Add and Done on a WaitGroup struct within a variable named differently than wg will be ignored
// This simplification avoids requiring type information while still makes the rule work in most of the cases.
// This rule assumes the WaitGroup variable is named 'wg', which is the common convention.
func (w *lintUseWaitGroupGo) analyzeBlock(b *ast.BlockStmt) {
	// we will iterate over all statements in search for wg.Add()
	stmts := b.List
	for i := 0; i < len(stmts); i++ {
		stmt := stmts[i]
		if !w.isCallToWgAdd(stmt) {
			continue
		}

		call := stmt

		// Here we have identified a call to wg.Add
		// Let's iterate over the statements that follow the wg.Add
		// to see if there is a go statement that runs a goroutine with a wg.Done
		//
		// wg.Add is the i-th statement of block.List
		// we will iterate from the (i+1)-th statement up to the last statement of block.List
		for i++; i < len(stmts); i++ {
			stmt := stmts[i]
			// looking for a go statement
			goStmt, ok := stmt.(*ast.GoStmt)
			if !ok {
				continue // not a go statement
			}

			// here we found a the go statement
			// now let's check is the go statement is applied to a function literal that contains a wg.Done
			if !w.hasCallToWgDone(goStmt) {
				continue
			}

			w.onFailure(lint.Failure{
				Confidence: 1,
				Node:       call,
				Category:   lint.FailureCategoryStyle,
				Failure:    "replace wg.Add()...go {...wg.Done()...} with wg.Go(...)",
			})

			break
		}
	}
}

// hasCallToWgDone returns true if the given go statement
// calls to a function literal containing a call to wg.Done, false otherwise.
func (*lintUseWaitGroupGo) hasCallToWgDone(goStmt *ast.GoStmt) bool {
	funcLit, ok := goStmt.Call.Fun.(*ast.FuncLit)
	if !ok {
		return false // the go statements runs a function defined elsewhere
	}

	// here we found a go statement running a function literal
	// now we will look for a wg.Done inside the body of the function literal
	wgDoneStmt := astutils.SeekNode[ast.Node](funcLit.Body, wgDonePicker)

	return wgDoneStmt != nil
}

// isCallToWgAdd returns true if the given statement is a call to wg.Add, false otherwise.
func (*lintUseWaitGroupGo) isCallToWgAdd(stmt ast.Stmt) bool {
	expr, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return false // not an expression statements thus not a function call
	}

	// Lets check if the expression statement is a call to wg.Add
	call, ok := expr.X.(*ast.CallExpr)

	return ok && astutils.IsPkgDotName(call.Fun, "wg", "Add")
}

// wgDonePicker is used when calling astutils.SeekNode that search for calls to wg.Done.
func wgDonePicker(n ast.Node) bool {
	call, ok := n.(*ast.CallExpr)
	result := ok && astutils.IsPkgDotName(call.Fun, "wg", "Done")
	return result
}
