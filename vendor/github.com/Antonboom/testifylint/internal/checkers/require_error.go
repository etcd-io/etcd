package checkers

import (
	"go/ast"
	"regexp"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"
)

const requireErrorReport = "for error assertions use require"

// RequireError detects error assertions like
//
//	assert.Error(t, err) // s.Error(err), s.Assert().Error(err)
//	assert.ErrorIs(t, err, io.EOF)
//	assert.ErrorAs(t, err, &target)
//	assert.EqualError(t, err, "end of file")
//	assert.ErrorContains(t, err, "end of file")
//	assert.NoError(t, err)
//	assert.NotErrorIs(t, err, io.EOF)
//
// and requires
//
//	require.Error(t, err) // s.Require().Error(err), s.Require().Error(err)
//	require.ErrorIs(t, err, io.EOF)
//	require.ErrorAs(t, err, &target)
//	...
//
// RequireError ignores:
// - assertions in the `if` condition;
// - assertions in the bool expression;
// - the entire `if-else[-if]` block, if there is an assertion in any `if` condition;
// - the last assertion in the block, if there are no methods/functions calls after it;
// - assertions in an explicit goroutine (including `http.Handler`);
// - assertions in an explicit testing cleanup function or suite teardown methods;
// - sequence of NoError assertions.
type RequireError struct {
	fnPattern *regexp.Regexp
}

// NewRequireError constructs RequireError checker.
func NewRequireError() *RequireError { return new(RequireError) }
func (RequireError) Name() string    { return "require-error" }

func (checker *RequireError) SetFnPattern(p *regexp.Regexp) *RequireError {
	if p != nil {
		checker.fnPattern = p
	}
	return checker
}

func (checker RequireError) Check(pass *analysis.Pass, insp *inspector.Inspector) []analysis.Diagnostic {
	callsByFunc := make(map[funcID][]*callMeta)

	// Stage 1. Collect meta information about any calls inside functions.

	insp.WithStack([]ast.Node{(*ast.CallExpr)(nil)}, func(node ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return false
		}
		if len(stack) < 3 {
			return true
		}

		fID := findSurroundingFunc(pass, stack)
		if fID == nil {
			return true
		}

		_, prevIsIfStmt := stack[len(stack)-2].(*ast.IfStmt)
		_, prevIsAssignStmt := stack[len(stack)-2].(*ast.AssignStmt)
		_, prevPrevIsIfStmt := stack[len(stack)-3].(*ast.IfStmt)
		inIfCond := prevIsIfStmt || (prevPrevIsIfStmt && prevIsAssignStmt)

		_, inBoolExpr := stack[len(stack)-2].(*ast.BinaryExpr)

		callExpr := node.(*ast.CallExpr)
		testifyCall := NewCallMeta(pass, callExpr)

		call := &callMeta{
			call:         callExpr,
			testifyCall:  testifyCall,
			rootIf:       findRootIf(stack),
			parentIf:     findNearestNode[*ast.IfStmt](stack),
			parentBlock:  findNearestNode[*ast.BlockStmt](stack),
			inIfCond:     inIfCond,
			inBoolExpr:   inBoolExpr,
			inNoErrorSeq: false, // Will be filled in below.
		}

		callsByFunc[*fID] = append(callsByFunc[*fID], call)
		return testifyCall == nil // Do not support asserts in asserts.
	})

	// Stage 2. Analyze calls and block context.

	var diagnostics []analysis.Diagnostic

	callsByBlock := map[*ast.BlockStmt][]*callMeta{}
	for _, calls := range callsByFunc {
		for _, c := range calls {
			if b := c.parentBlock; b != nil {
				callsByBlock[b] = append(callsByBlock[b], c)
			}
		}
	}

	markCallsInNoErrorSequence(callsByBlock)

	for funcInfo, calls := range callsByFunc {
		for i, c := range calls {
			if m := funcInfo.meta; m.isTestCleanup || m.isGoroutine || m.isHTTPHandler {
				continue
			}

			if c.testifyCall == nil {
				continue
			}
			if !c.testifyCall.IsAssert {
				continue
			}
			switch c.testifyCall.Fn.NameFTrimmed {
			default:
				continue
			case "Error", "ErrorIs", "ErrorAs", "EqualError", "ErrorContains", "NoError", "NotErrorIs":
			}

			if needToSkipBasedOnContext(c, i, calls, callsByBlock) {
				continue
			}
			if p := checker.fnPattern; p != nil && !p.MatchString(c.testifyCall.Fn.Name) {
				continue
			}

			diagnostics = append(diagnostics,
				*newDiagnostic(checker.Name(), c.testifyCall, requireErrorReport))
		}
	}

	return diagnostics
}

func needToSkipBasedOnContext(
	currCall *callMeta,
	currCallIndex int,
	otherCalls []*callMeta,
	callsByBlock map[*ast.BlockStmt][]*callMeta,
) bool {
	if currCall.inIfCond || currCall.inBoolExpr || currCall.inNoErrorSeq {
		return true
	}

	if currCall.rootIf != nil {
		for _, rootCall := range otherCalls {
			if (rootCall.rootIf == currCall.rootIf) && rootCall.inIfCond {
				// Skip assertions in the entire if-else[-if] block, if some of "if condition" contains assertion.
				return true
			}
		}
	}

	block := currCall.parentBlock
	blockCalls := callsByBlock[block]
	isLastCallInBlock := blockCalls[len(blockCalls)-1] == currCall

	noCallsAfter := true

	_, blockEndWithReturn := block.List[len(block.List)-1].(*ast.ReturnStmt)
	if !blockEndWithReturn {
		for i := currCallIndex + 1; i < len(otherCalls); i++ {
			nextCall := otherCalls[i]
			nextCallInElseBlock := false

			if pIf := currCall.parentIf; pIf != nil && pIf.Else != nil {
				ast.Inspect(pIf.Else, func(n ast.Node) bool {
					if n == nextCall.call {
						nextCallInElseBlock = true
						return false
					}
					return true
				})
			}

			if !nextCallInElseBlock {
				noCallsAfter = false
				break
			}
		}
	}

	// Skip assertion if this is the last operation in the test.
	return isLastCallInBlock && noCallsAfter
}

func findRootIf(stack []ast.Node) *ast.IfStmt {
	nearestIf, i := findNearestNodeWithIdx[*ast.IfStmt](stack)
	for ; i > 0; i-- {
		parent, ok := stack[i-1].(*ast.IfStmt)
		if !ok {
			break
		}
		nearestIf = parent
	}
	return nearestIf
}

func markCallsInNoErrorSequence(callsByBlock map[*ast.BlockStmt][]*callMeta) {
	for _, calls := range callsByBlock {
		for i, c := range calls {
			if c.testifyCall == nil {
				continue
			}

			var prevIsNoError bool
			if i > 0 {
				if prev := calls[i-1].testifyCall; prev != nil {
					prevIsNoError = isNoErrorAssertion(prev.Fn.Name)
				}
			}

			var nextIsNoError bool
			if i < len(calls)-1 {
				if next := calls[i+1].testifyCall; next != nil {
					nextIsNoError = isNoErrorAssertion(next.Fn.Name)
				}
			}

			if isNoErrorAssertion(c.testifyCall.Fn.Name) && (prevIsNoError || nextIsNoError) {
				calls[i].inNoErrorSeq = true
			}
		}
	}
}

type callMeta struct {
	call         *ast.CallExpr
	testifyCall  *CallMeta
	rootIf       *ast.IfStmt // The root `if` in if-else[-if] chain.
	parentIf     *ast.IfStmt // The nearest `if`, can be equal with rootIf.
	parentBlock  *ast.BlockStmt
	inIfCond     bool // True for code like `if assert.ErrorAs(t, err, &target) {`.
	inBoolExpr   bool // True for code like `assert.Error(t, err) && assert.ErrorContains(t, err, "value")`
	inNoErrorSeq bool // True for sequence of `assert.NoError` assertions.
}

func isNoErrorAssertion(fnName string) bool {
	return (fnName == "NoError") || (fnName == "NoErrorf")
}
