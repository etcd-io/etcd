package paralleltest

import (
	"flag"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"
)

const Doc = `check that tests use t.Parallel() method
It also checks that the t.Parallel is used if multiple tests cases are run as part of single test.
As part of ensuring parallel tests works as expected it checks for reinitializing of the range value
over the test cases.(https://tinyurl.com/y6555cy6)
With the -checkcleanup flag, it also checks that defer is not used with t.Parallel (use t.Cleanup instead).`

func NewAnalyzer() *analysis.Analyzer {
	return newParallelAnalyzer().analyzer
}

// parallelAnalyzer is an internal analyzer that makes options available to a
// run pass. It wraps an `analysis.Analyzer` that should be returned for
// linters.
type parallelAnalyzer struct {
	analyzer              *analysis.Analyzer
	ignoreMissing         bool
	ignoreMissingSubtests bool
	ignoreLoopVar         bool
	checkCleanup          bool
}

func newParallelAnalyzer() *parallelAnalyzer {
	a := &parallelAnalyzer{}

	var flags flag.FlagSet
	flags.BoolVar(&a.ignoreMissing, "i", false, "ignore missing calls to t.Parallel")
	flags.BoolVar(&a.ignoreMissingSubtests, "ignoremissingsubtests", false, "ignore missing calls to t.Parallel in subtests")
	flags.BoolVar(&a.ignoreLoopVar, "ignoreloopVar", false, "ignore loop variable detection")
	flags.BoolVar(&a.checkCleanup, "checkcleanup", false, "check that defer is not used with t.Parallel (use t.Cleanup instead)")

	a.analyzer = &analysis.Analyzer{
		Name:  "paralleltest",
		Doc:   Doc,
		Run:   a.run,
		Flags: flags,
	}
	return a
}

type testFunctionAnalysis struct {
	funcHasParallelMethod,
	funcCantParallelMethod,
	rangeStatementOverTestCasesExists,
	rangeStatementHasParallelMethod,
	rangeStatementCantParallelMethod,
	funcHasDeferStatement bool
	loopVariableUsedInRun *string
	numberOfTestRun       int
	positionOfTestRunNode []ast.Node
	rangeNode             ast.Node
	deferStatements       []ast.Node
}

type testRunAnalysis struct {
	hasParallel           bool
	cantParallel          bool
	numberOfTestRun       int
	positionOfTestRunNode []ast.Node
}

func (a *parallelAnalyzer) analyzeTestRun(pass *analysis.Pass, n ast.Node, testVar string) testRunAnalysis {
	var analysis testRunAnalysis

	if methodRunIsCalledInTestFunction(n, testVar) {
		innerTestVar := getRunCallbackParameterName(n)
		analysis.numberOfTestRun++

		if callExpr, ok := n.(*ast.CallExpr); ok && len(callExpr.Args) > 1 {
			if funcLit, ok := callExpr.Args[1].(*ast.FuncLit); ok {
				ast.Inspect(funcLit, func(p ast.Node) bool {
					if !analysis.hasParallel {
						analysis.hasParallel = methodParallelIsCalledInTestFunction(p, innerTestVar)
					}
					if !analysis.cantParallel {
						analysis.cantParallel = methodSetenvIsCalledInTestFunction(p, innerTestVar)
					}
					return true
				})
			} else if ident, ok := callExpr.Args[1].(*ast.Ident); ok {
				// Case 2: Direct function identifier: t.Run("name", myFunc)
				foundFunc := false
				for _, file := range pass.Files {
					for _, decl := range file.Decls {
						if funcDecl, ok := decl.(*ast.FuncDecl); ok && funcDecl.Name.Name == ident.Name {
							foundFunc = true
							isReceivingTestContext, testParamName := isFunctionReceivingTestContext(funcDecl)
							if isReceivingTestContext {
								ast.Inspect(funcDecl, func(p ast.Node) bool {
									if !analysis.hasParallel {
										analysis.hasParallel = methodParallelIsCalledInTestFunction(p, testParamName)
									}
									return true
								})
							}
						}
					}
				}
				if !foundFunc {
					analysis.hasParallel = false
				}
			} else if builderCall, ok := callExpr.Args[1].(*ast.CallExpr); ok {
				// Case 3: Function call that returns a function: t.Run("name", builder())
				analysis.hasParallel = a.checkBuilderFunctionForParallel(pass, builderCall)
			}
		}

		if !analysis.hasParallel && !analysis.cantParallel {
			analysis.positionOfTestRunNode = append(analysis.positionOfTestRunNode, n)
		}
	}

	return analysis
}

func (a *parallelAnalyzer) analyzeTestFunction(pass *analysis.Pass, funcDecl *ast.FuncDecl) {
	var analysis testFunctionAnalysis

	// Check runs for test functions only
	isTest, testVar := isTestFunction(funcDecl)
	if !isTest {
		return
	}

	for _, l := range funcDecl.Body.List {
		switch v := l.(type) {
		case *ast.DeferStmt:
			if a.checkCleanup {
				analysis.funcHasDeferStatement = true
				analysis.deferStatements = append(analysis.deferStatements, v)
			}

		case *ast.ExprStmt:
			ast.Inspect(v, func(n ast.Node) bool {
				if !analysis.funcHasParallelMethod {
					analysis.funcHasParallelMethod = methodParallelIsCalledInTestFunction(n, testVar)
				}
				if !analysis.funcCantParallelMethod {
					analysis.funcCantParallelMethod = methodSetenvIsCalledInTestFunction(n, testVar)
				}
				runAnalysis := a.analyzeTestRun(pass, n, testVar)
				analysis.numberOfTestRun += runAnalysis.numberOfTestRun
				analysis.positionOfTestRunNode = append(analysis.positionOfTestRunNode, runAnalysis.positionOfTestRunNode...)
				return true
			})

		case *ast.RangeStmt:
			analysis.rangeNode = v

			var loopVars []types.Object
			for _, expr := range []ast.Expr{v.Key, v.Value} {
				if id, ok := expr.(*ast.Ident); ok {
					loopVars = append(loopVars, pass.TypesInfo.ObjectOf(id))
				}
			}

			ast.Inspect(v, func(n ast.Node) bool {
				if r, ok := n.(*ast.ExprStmt); ok {
					if methodRunIsCalledInRangeStatement(r.X, testVar) {
						innerTestVar := getRunCallbackParameterName(r.X)
						analysis.rangeStatementOverTestCasesExists = true

						if !analysis.rangeStatementHasParallelMethod {
							analysis.rangeStatementHasParallelMethod = methodParallelIsCalledInMethodRun(r.X, innerTestVar)
						}
						if !analysis.rangeStatementCantParallelMethod {
							analysis.rangeStatementCantParallelMethod = methodSetenvIsCalledInMethodRun(r.X, innerTestVar)
						}
						if !a.ignoreLoopVar && analysis.loopVariableUsedInRun == nil {
							if run, ok := r.X.(*ast.CallExpr); ok {
								analysis.loopVariableUsedInRun = loopVarReferencedInRun(run, loopVars, pass.TypesInfo)
							}
						}

						// Check nested test runs
						if callExpr, ok := r.X.(*ast.CallExpr); ok && len(callExpr.Args) > 1 {
							if funcLit, ok := callExpr.Args[1].(*ast.FuncLit); ok {
								ast.Inspect(funcLit, func(p ast.Node) bool {
									runAnalysis := a.analyzeTestRun(pass, p, innerTestVar)
									analysis.numberOfTestRun += runAnalysis.numberOfTestRun
									analysis.positionOfTestRunNode = append(analysis.positionOfTestRunNode, runAnalysis.positionOfTestRunNode...)
									return true
								})
							}
						}
					}
				}
				return true
			})
		}
	}

	if analysis.rangeStatementCantParallelMethod {
		analysis.funcCantParallelMethod = true
	}

	if !a.ignoreMissing && !analysis.funcHasParallelMethod && !analysis.funcCantParallelMethod {
		pass.Reportf(funcDecl.Pos(), "Function %s missing the call to method parallel\n", funcDecl.Name.Name)
	}

	if analysis.rangeStatementOverTestCasesExists && analysis.rangeNode != nil {
		if !analysis.rangeStatementHasParallelMethod && !analysis.rangeStatementCantParallelMethod {
			if !a.ignoreMissing && !a.ignoreMissingSubtests {
				pass.Reportf(analysis.rangeNode.Pos(), "Range statement for test %s missing the call to method parallel in test Run\n", funcDecl.Name.Name)
			}
		} else if analysis.loopVariableUsedInRun != nil && !a.ignoreLoopVar {
			pass.Reportf(analysis.rangeNode.Pos(), "Range statement for test %s does not reinitialise the variable %s\n", funcDecl.Name.Name, *analysis.loopVariableUsedInRun)
		}
	}

	if !a.ignoreMissing && !a.ignoreMissingSubtests {
		if analysis.numberOfTestRun > 1 && len(analysis.positionOfTestRunNode) > 0 {
			for _, n := range analysis.positionOfTestRunNode {
				pass.Reportf(n.Pos(), "Function %s missing the call to method parallel in the test run\n", funcDecl.Name.Name)
			}
		}
	}

	if a.checkCleanup && analysis.funcHasParallelMethod && analysis.funcHasDeferStatement {
		for _, deferStmt := range analysis.deferStatements {
			pass.Reportf(deferStmt.Pos(), "Function %s uses defer with t.Parallel, use t.Cleanup instead to ensure cleanup runs after parallel subtests complete", funcDecl.Name.Name)
		}
	}
}

// checkBuilderFunctionForParallel analyzes a function call that returns a test function
// to see if the returned function contains t.Parallel()
func (a *parallelAnalyzer) checkBuilderFunctionForParallel(pass *analysis.Pass, builderCall *ast.CallExpr) bool {
	// Get the name of the builder function being called
	var builderFuncName string
	switch fun := builderCall.Fun.(type) {
	case *ast.Ident:
		builderFuncName = fun.Name
	case *ast.SelectorExpr:
		// Handle method calls like obj.Builder()
		builderFuncName = fun.Sel.Name
	default:
		return false
	}

	if builderFuncName == "" {
		return false
	}

	// Find the builder function declaration
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok || funcDecl.Name.Name != builderFuncName {
				continue
			}

			// Found the builder function, analyze it and return immediately
			hasParallel := false
			ast.Inspect(funcDecl, func(n ast.Node) bool {
				// Look for return statements
				returnStmt, ok := n.(*ast.ReturnStmt)
				if !ok || len(returnStmt.Results) == 0 {
					return true
				}

				// Check if the return value is a function literal
				for _, result := range returnStmt.Results {
					if funcLit, ok := result.(*ast.FuncLit); ok {
						// Get the parameter name from the returned function
						var paramName string
						if funcLit.Type != nil && funcLit.Type.Params != nil && len(funcLit.Type.Params.List) > 0 {
							param := funcLit.Type.Params.List[0]
							if len(param.Names) > 0 {
								paramName = param.Names[0].Name
							}
						}

						// Inspect the returned function for t.Parallel()
						if paramName != "" {
							ast.Inspect(funcLit, func(p ast.Node) bool {
								if methodParallelIsCalledInTestFunction(p, paramName) {
									hasParallel = true
									return false
								}
								return true
							})

							// Exit inspection immediately if we found t.Parallel()
							if hasParallel {
								return false
							}
						}
					}
				}
				// Continue to next return statement if t.Parallel() not found yet
				return true
			})

			// Return immediately after processing the matching function
			return hasParallel
		}
	}

	return false
}

func (a *parallelAnalyzer) run(pass *analysis.Pass) (interface{}, error) {
	inspector := inspector.New(pass.Files)

	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
	}

	inspector.Preorder(nodeFilter, func(node ast.Node) {
		funcDecl := node.(*ast.FuncDecl)
		// Only process _test.go files
		if !strings.HasSuffix(pass.Fset.File(funcDecl.Pos()).Name(), "_test.go") {
			return
		}
		a.analyzeTestFunction(pass, funcDecl)
	})

	return nil, nil
}

func methodParallelIsCalledInMethodRun(node ast.Node, testVar string) bool {
	return targetMethodIsCalledInMethodRun(node, testVar, "Parallel")
}

func methodSetenvIsCalledInMethodRun(node ast.Node, testVar string) bool {
	return targetMethodIsCalledInMethodRun(node, testVar, "Setenv")
}

func targetMethodIsCalledInMethodRun(node ast.Node, testVar, targetMethod string) bool {
	var called bool
	// nolint: gocritic
	switch callExp := node.(type) {
	case *ast.CallExpr:
		for _, arg := range callExp.Args {
			if !called {
				ast.Inspect(arg, func(n ast.Node) bool {
					if !called {
						called = exprCallHasMethod(n, testVar, targetMethod)
						return true
					}
					return false
				})
			}
		}
	}
	return called
}

func methodParallelIsCalledInTestFunction(node ast.Node, testVar string) bool {
	return exprCallHasMethod(node, testVar, "Parallel")
}

func methodRunIsCalledInRangeStatement(node ast.Node, testVar string) bool {
	return exprCallHasMethod(node, testVar, "Run")
}

func methodRunIsCalledInTestFunction(node ast.Node, testVar string) bool {
	return exprCallHasMethod(node, testVar, "Run")
}

func methodSetenvIsCalledInTestFunction(node ast.Node, testVar string) bool {
	return exprCallHasMethod(node, testVar, "Setenv")
}

func exprCallHasMethod(node ast.Node, receiverName, methodName string) bool {
	// nolint: gocritic
	switch n := node.(type) {
	case *ast.CallExpr:
		if fun, ok := n.Fun.(*ast.SelectorExpr); ok {
			if receiver, ok := fun.X.(*ast.Ident); ok {
				return receiver.Name == receiverName && fun.Sel.Name == methodName
			}
		}
	}
	return false
}

// In an expression of the form t.Run(x, func(q *testing.T) {...}), return the
// value "q". In _most_ code, the name is probably t, but we shouldn't just
// assume.
func getRunCallbackParameterName(node ast.Node) string {
	if n, ok := node.(*ast.CallExpr); ok {
		if len(n.Args) < 2 {
			// We want argument #2, but this call doesn't have two
			// arguments. Maybe it's not really t.Run.
			return ""
		}
		funcArg := n.Args[1]
		if fun, ok := funcArg.(*ast.FuncLit); ok {
			if len(fun.Type.Params.List) < 1 {
				// Subtest function doesn't have any parameters.
				return ""
			}
			firstArg := fun.Type.Params.List[0]
			// We'll assume firstArg.Type is *testing.T.
			if len(firstArg.Names) < 1 {
				return ""
			}
			return firstArg.Names[0].Name
		}
	}
	return ""
}

// isFunctionReceivingTestContext checks if a function declaration receives a *testing.T parameter
// Returns (true, paramName) if it does, (false, "") if it doesn't
func isFunctionReceivingTestContext(funcDecl *ast.FuncDecl) (bool, string) {
	testMethodPackageType := "testing"
	testMethodStruct := "T"

	if funcDecl.Type.Params != nil && len(funcDecl.Type.Params.List) != 1 {
		return false, ""
	}

	param := funcDecl.Type.Params.List[0]
	if starExp, ok := param.Type.(*ast.StarExpr); ok {
		if selectExpr, ok := starExp.X.(*ast.SelectorExpr); ok {
			if selectExpr.Sel.Name == testMethodStruct {
				if s, ok := selectExpr.X.(*ast.Ident); ok {
					if len(param.Names) > 0 {
						return s.Name == testMethodPackageType, param.Names[0].Name
					}
				}
			}
		}
	}

	return false, ""
}

// isTestFunction checks if a function declaration is a test function
// A test function must:
// 1. Start with "Test"
// 2. Have exactly one parameter
// 3. Have that parameter be of type *testing.T
// Returns (true, paramName) if it is a test function, (false, "") if it isn't
func isTestFunction(funcDecl *ast.FuncDecl) (bool, string) {
	testMethodPackageType := "testing"
	testMethodStruct := "T"
	testPrefix := "Test"

	if !strings.HasPrefix(funcDecl.Name.Name, testPrefix) {
		return false, ""
	}

	if funcDecl.Type.Params != nil && len(funcDecl.Type.Params.List) != 1 {
		return false, ""
	}

	param := funcDecl.Type.Params.List[0]
	if starExp, ok := param.Type.(*ast.StarExpr); ok {
		if selectExpr, ok := starExp.X.(*ast.SelectorExpr); ok {
			if selectExpr.Sel.Name == testMethodStruct {
				if s, ok := selectExpr.X.(*ast.Ident); ok {
					if len(param.Names) > 0 {
						return s.Name == testMethodPackageType, param.Names[0].Name
					}
				}
			}
		}
	}

	return false, ""
}

// loopVarReferencedInRun checks if a loop variable is referenced within a test run
// This is important for detecting potential race conditions in parallel tests
func loopVarReferencedInRun(call *ast.CallExpr, vars []types.Object, typeInfo *types.Info) (found *string) {
	if len(call.Args) != 2 {
		return
	}

	ast.Inspect(call.Args[1], func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		for _, o := range vars {
			if typeInfo.ObjectOf(ident) == o {
				found = &ident.Name
			}
		}
		return true
	})

	return
}
