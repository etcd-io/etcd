// Package analyzer implements the thelper linter logic.
package analyzer

import (
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	doc = "thelper detects tests helpers which do not start with the t.Helper() method."

	checksDoc = `coma separated list of enabled checks

Available checks

` + checkTBegin + ` - check t.Helper() begins helper function
` + checkTFirst + ` - check *testing.T is first param of helper function
` + checkTName + `  - check *testing.T param has t name

Also available similar checks for benchmark and TB helpers: ` +
		checkFBegin + `, ` + checkFFirst + `, ` + checkFName + `,` +
		checkBBegin + `, ` + checkBFirst + `, ` + checkBName + `,` +
		checkTBBegin + `, ` + checkTBFirst + `, ` + checkTBName + `

`
)

type enabledChecksValue map[string]struct{}

func (m enabledChecksValue) Enabled(c string) bool {
	_, ok := m[c]
	return ok
}

func (m enabledChecksValue) String() string {
	ss := make([]string, 0, len(m))
	for s := range m {
		ss = append(ss, s)
	}

	sort.Strings(ss)

	return strings.Join(ss, ",")
}

func (m enabledChecksValue) Set(s string) error {
	ss := strings.FieldsFunc(s, func(c rune) bool { return c == ',' })
	if len(ss) == 0 {
		return nil
	}

	for k := range m {
		delete(m, k)
	}

	for _, v := range ss {
		switch v {
		case checkTBegin, checkTFirst, checkTName,
			checkFBegin, checkFFirst, checkFName,
			checkBBegin, checkBFirst, checkBName,
			checkTBBegin, checkTBFirst, checkTBName:
			m[v] = struct{}{}
		default:
			return fmt.Errorf("unknown check name %q (see help for full list)", v)
		}
	}

	return nil
}

const (
	checkTBegin  = "t_begin"
	checkTFirst  = "t_first"
	checkTName   = "t_name"
	checkFBegin  = "f_begin"
	checkFFirst  = "f_first"
	checkFName   = "f_name"
	checkBBegin  = "b_begin"
	checkBFirst  = "b_first"
	checkBName   = "b_name"
	checkTBBegin = "tb_begin"
	checkTBFirst = "tb_first"
	checkTBName  = "tb_name"
)

type thelper struct {
	enabledChecks enabledChecksValue
}

// NewAnalyzer return a new thelper analyzer.
// thelper analyzes Go test codes how they use t.Helper() method.
func NewAnalyzer() *analysis.Analyzer {
	thelper := thelper{}
	thelper.enabledChecks = enabledChecksValue{
		checkTBegin:  struct{}{},
		checkTFirst:  struct{}{},
		checkTName:   struct{}{},
		checkFBegin:  struct{}{},
		checkFFirst:  struct{}{},
		checkFName:   struct{}{},
		checkBBegin:  struct{}{},
		checkBFirst:  struct{}{},
		checkBName:   struct{}{},
		checkTBBegin: struct{}{},
		checkTBFirst: struct{}{},
		checkTBName:  struct{}{},
	}

	a := &analysis.Analyzer{
		Name: "thelper",
		Doc:  doc,
		Run:  thelper.run,
		Requires: []*analysis.Analyzer{
			inspect.Analyzer,
		},
	}

	a.Flags.Init("thelper", flag.ExitOnError)
	a.Flags.Var(&thelper.enabledChecks, "checks", checksDoc)

	return a
}

//nolint:funlen // The function is easier to grok this way.
func (t thelper) run(pass *analysis.Pass) (interface{}, error) {
	tCheckOpts, fCheckOpts, bCheckOpts, tbCheckOpts, ok := t.buildCheckFuncOpts(pass)
	if !ok {
		return nil, nil
	}

	inspect, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, nil
	}

	var reports reports

	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
		(*ast.FuncLit)(nil),
		(*ast.CallExpr)(nil),
	}
	inspect.Preorder(nodeFilter, func(node ast.Node) {
		var fd funcDecl

		switch n := node.(type) {
		case *ast.FuncLit:
			fd.Pos = n.Pos()
			fd.Type = n.Type
			fd.Body = n.Body
			fd.Name = ast.NewIdent("")
		case *ast.FuncDecl:
			fd.Pos = n.Name.NamePos
			fd.Type = n.Type
			fd.Body = n.Body
			fd.Name = n.Name
		case *ast.CallExpr:
			runSubtestExprs := extractSubtestExp(pass, n, tCheckOpts.subRun, tCheckOpts.subTestFuncType)

			if len(runSubtestExprs) == 0 {
				runSubtestExprs = extractSubtestExp(pass, n, bCheckOpts.subRun, bCheckOpts.subTestFuncType)
			}

			if len(runSubtestExprs) == 0 {
				runSubtestExprs = extractSubtestFuzzExp(pass, n, fCheckOpts.subRun)
			}

			if len(runSubtestExprs) == 0 {
				runSubtestExprs = extractSynctestExp(pass, n, tCheckOpts.subTestFuncType)
			}

			if len(runSubtestExprs) > 0 {
				for _, expr := range runSubtestExprs {
					reports.Filter(funcDefPosition(pass, expr))
				}
			} else {
				reports.NoFilter(funcDefPosition(pass, n.Fun))
			}

			return
		default:
			return
		}

		checkFunc(pass, &reports, fd, tCheckOpts)
		checkFunc(pass, &reports, fd, fCheckOpts)
		checkFunc(pass, &reports, fd, bCheckOpts)
		checkFunc(pass, &reports, fd, tbCheckOpts)
	})

	reports.Flush(pass)

	return nil, nil
}

type checkFuncOpts struct {
	skipPrefix      string
	varName         string
	fnHelper        types.Object
	subRun          types.Object
	subTestFuncType types.Type
	hpType          types.Type
	ctxType         types.Type
	checkBegin      bool
	checkFirst      bool
	checkName       bool
}

func (t thelper) buildCheckFuncOpts(pass *analysis.Pass) (checkFuncOpts, checkFuncOpts, checkFuncOpts, checkFuncOpts, bool) {
	var ctxType types.Type

	ctxObj := findTypeObject(pass, "context.Context")
	if ctxObj != nil {
		ctxType = ctxObj.Type()
	}

	tCheckOpts, ok := t.buildTestCheckFuncOpts(pass, ctxType)
	if !ok {
		return checkFuncOpts{}, checkFuncOpts{}, checkFuncOpts{}, checkFuncOpts{}, false
	}

	fCheckOpts, ok := t.buildFuzzCheckFuncOpts(pass, ctxType)
	if !ok {
		return checkFuncOpts{}, checkFuncOpts{}, checkFuncOpts{}, checkFuncOpts{}, false
	}

	bCheckOpts, ok := t.buildBenchmarkCheckFuncOpts(pass, ctxType)
	if !ok {
		return checkFuncOpts{}, checkFuncOpts{}, checkFuncOpts{}, checkFuncOpts{}, false
	}

	tbCheckOpts, ok := t.buildTBCheckFuncOpts(pass, ctxType)
	if !ok {
		return checkFuncOpts{}, checkFuncOpts{}, checkFuncOpts{}, checkFuncOpts{}, false
	}

	return tCheckOpts, fCheckOpts, bCheckOpts, tbCheckOpts, true
}

func (t thelper) buildTestCheckFuncOpts(pass *analysis.Pass, ctxType types.Type) (checkFuncOpts, bool) {
	tObj := findTypeObject(pass, "testing.T")
	if tObj == nil {
		return checkFuncOpts{}, false
	}

	tHelper, _, _ := types.LookupFieldOrMethod(tObj.Type(), true, tObj.Pkg(), "Helper")
	if tHelper == nil {
		return checkFuncOpts{}, false
	}

	tRun, _, _ := types.LookupFieldOrMethod(tObj.Type(), true, tObj.Pkg(), "Run")
	if tRun == nil {
		return checkFuncOpts{}, false
	}

	tType := types.NewPointer(tObj.Type())
	tVar := types.NewVar(token.NoPos, nil, "t", tType)

	return checkFuncOpts{
		skipPrefix:      "Test",
		varName:         "t",
		fnHelper:        tHelper,
		subRun:          tRun,
		hpType:          tType,
		subTestFuncType: types.NewSignatureType(nil, nil, nil, types.NewTuple(tVar), nil, false),
		ctxType:         ctxType,
		checkBegin:      t.enabledChecks.Enabled(checkTBegin),
		checkFirst:      t.enabledChecks.Enabled(checkTFirst),
		checkName:       t.enabledChecks.Enabled(checkTName),
	}, true
}

func (t thelper) buildFuzzCheckFuncOpts(pass *analysis.Pass, ctxType types.Type) (checkFuncOpts, bool) {
	fObj := findTypeObject(pass, "testing.F")
	if fObj == nil {
		return checkFuncOpts{}, true // fuzzing supports since go1.18, it's ok, that testig.F is missed.
	}

	fHelper, _, _ := types.LookupFieldOrMethod(fObj.Type(), true, fObj.Pkg(), "Helper")
	if fHelper == nil {
		return checkFuncOpts{}, false
	}

	tFuzz, _, _ := types.LookupFieldOrMethod(fObj.Type(), true, fObj.Pkg(), "Fuzz")
	if tFuzz == nil {
		return checkFuncOpts{}, false
	}

	return checkFuncOpts{
		skipPrefix: "Fuzz",
		varName:    "f",
		fnHelper:   fHelper,
		subRun:     tFuzz,
		hpType:     types.NewPointer(fObj.Type()),
		ctxType:    ctxType,
		checkBegin: t.enabledChecks.Enabled(checkFBegin),
		checkFirst: t.enabledChecks.Enabled(checkFFirst),
		checkName:  t.enabledChecks.Enabled(checkFName),
	}, true
}

func (t thelper) buildBenchmarkCheckFuncOpts(pass *analysis.Pass, ctxType types.Type) (checkFuncOpts, bool) {
	bObj := findTypeObject(pass, "testing.B")
	if bObj == nil {
		return checkFuncOpts{}, false
	}

	bHelper, _, _ := types.LookupFieldOrMethod(bObj.Type(), true, bObj.Pkg(), "Helper")
	if bHelper == nil {
		return checkFuncOpts{}, false
	}

	bRun, _, _ := types.LookupFieldOrMethod(bObj.Type(), true, bObj.Pkg(), "Run")
	if bRun == nil {
		return checkFuncOpts{}, false
	}

	bType := types.NewPointer(bObj.Type())
	bVar := types.NewVar(token.NoPos, nil, "b", bType)

	return checkFuncOpts{
		skipPrefix:      "Benchmark",
		varName:         "b",
		fnHelper:        bHelper,
		subRun:          bRun,
		hpType:          types.NewPointer(bObj.Type()),
		subTestFuncType: types.NewSignatureType(nil, nil, nil, types.NewTuple(bVar), nil, false),
		ctxType:         ctxType,
		checkBegin:      t.enabledChecks.Enabled(checkBBegin),
		checkFirst:      t.enabledChecks.Enabled(checkBFirst),
		checkName:       t.enabledChecks.Enabled(checkBName),
	}, true
}

func (t thelper) buildTBCheckFuncOpts(pass *analysis.Pass, ctxType types.Type) (checkFuncOpts, bool) {
	tbObj := findTypeObject(pass, "testing.TB")
	if tbObj == nil {
		return checkFuncOpts{}, false
	}

	tbHelper, _, _ := types.LookupFieldOrMethod(tbObj.Type(), true, tbObj.Pkg(), "Helper")
	if tbHelper == nil {
		return checkFuncOpts{}, false
	}

	return checkFuncOpts{
		skipPrefix: "",
		varName:    "tb",
		fnHelper:   tbHelper,
		hpType:     tbObj.Type(),
		ctxType:    ctxType,
		checkBegin: t.enabledChecks.Enabled(checkTBBegin),
		checkFirst: t.enabledChecks.Enabled(checkTBFirst),
		checkName:  t.enabledChecks.Enabled(checkTBName),
	}, true
}

type funcDecl struct {
	Pos  token.Pos
	Name *ast.Ident
	Type *ast.FuncType
	Body *ast.BlockStmt
}

func checkFunc(pass *analysis.Pass, reports *reports, funcDecl funcDecl, opts checkFuncOpts) {
	if !opts.checkFirst && !opts.checkBegin && !opts.checkName {
		return
	}

	if opts.skipPrefix != "" && strings.HasPrefix(funcDecl.Name.Name, opts.skipPrefix) {
		return
	}

	p, pos, ok := searchFuncParam(pass, funcDecl, opts.hpType)
	if !ok {
		return
	}

	if opts.checkFirst {
		if pos != 0 {
			checkFirstPassed := false

			if pos == 1 && opts.ctxType != nil {
				_, pos, ok := searchFuncParam(pass, funcDecl, opts.ctxType)
				checkFirstPassed = ok && (pos == 0)
			}

			if !checkFirstPassed {
				reports.Reportf(funcDecl.Pos, "parameter %s should be the first or after context.Context", opts.hpType)
			}
		}
	}

	if len(p.Names) > 0 && p.Names[0].Name != "_" {
		if opts.checkName {
			if p.Names[0].Name != opts.varName {
				reports.Reportf(funcDecl.Pos, "parameter %s should have name %s", opts.hpType, opts.varName)
			}
		}

		if opts.checkBegin {
			if len(funcDecl.Body.List) == 0 || !isTHelperCall(pass, funcDecl.Body.List[0], opts.fnHelper) {
				reports.Reportf(funcDecl.Pos, "test helper function should start from %s.Helper()", opts.varName)
			}
		}
	}
}

// searchFuncParam search a function param with desired type.
// It returns the param field, its position, and true if something is found.
func searchFuncParam(pass *analysis.Pass, f funcDecl, p types.Type) (*ast.Field, int, bool) {
	for i, f := range f.Type.Params.List {
		if isExprHasType(pass, f.Type, p) {
			return f, i, true
		}
	}

	return nil, 0, false
}

// isTHelperCall returns true if provided statement 's' is t.Helper() or b.Helper() call.
func isTHelperCall(pass *analysis.Pass, s ast.Stmt, tHelper types.Object) bool {
	exprStmt, ok := s.(*ast.ExprStmt)
	if !ok {
		return false
	}

	callExpr, ok := exprStmt.X.(*ast.CallExpr)
	if !ok {
		return false
	}

	selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	return isSelectorCall(pass, selExpr, tHelper)
}

// extractSubtestExp analyzes that call expresion 'e' is t.Run or b.Run
// and returns subtest function.
func extractSubtestExp(
	pass *analysis.Pass, e *ast.CallExpr, tbRun types.Object, testFuncType types.Type,
) []ast.Expr {
	selExpr, ok := e.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	if !isSelectorCall(pass, selExpr, tbRun) {
		return nil
	}

	if len(e.Args) != 2 {
		return nil
	}

	if funcs := unwrapTestingFunctionBuilding(pass, e.Args[1], testFuncType); funcs != nil {
		return funcs
	}

	return []ast.Expr{e.Args[1]}
}

// extractSubtestFuzzExp analyzes that call expresion 'e' is f.Fuzz
// and returns subtest function.
func extractSubtestFuzzExp(
	pass *analysis.Pass, e *ast.CallExpr, fuzzRun types.Object,
) []ast.Expr {
	selExpr, ok := e.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	if !isSelectorCall(pass, selExpr, fuzzRun) {
		return nil
	}

	if len(e.Args) != 1 {
		return nil
	}

	return []ast.Expr{e.Args[0]}
}

// extractSynctestExp analyzes that call expression 'e' is synctest.Test
// and returns the test function.
func extractSynctestExp(
	pass *analysis.Pass, e *ast.CallExpr, testFuncType types.Type,
) []ast.Expr {
	// Check if this is a call to synctest.Test
	selExpr, ok := e.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	// Check if the selector is "Test"
	if selExpr.Sel.Name != "Test" {
		return nil
	}

	// Check if the package is synctest by looking at the identifier
	ident, ok := selExpr.X.(*ast.Ident)
	if !ok {
		return nil
	}

	if !isIdentPackageName(pass, ident, "testing/synctest") {
		return nil
	}

	// synctest.Test takes 2 arguments: t *testing.T, f func(*testing.T)
	if len(e.Args) != 2 {
		return nil
	}

	if funcs := unwrapTestingFunctionBuilding(pass, e.Args[1], testFuncType); funcs != nil {
		return funcs
	}

	return []ast.Expr{e.Args[1]}
}

// unwrapTestingFunctionConstruction checks that expresion is build testing functions
// and returns the result of building.
func unwrapTestingFunctionBuilding(pass *analysis.Pass, expr ast.Expr, testFuncType types.Type) []ast.Expr {
	callExpr, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil
	}

	var funcDecl funcDecl

	switch f := callExpr.Fun.(type) {
	case *ast.FuncLit:
		funcDecl.Body = f.Body
		funcDecl.Type = f.Type
	case *ast.Ident:
		funObjDecl := findFunctionDeclaration(pass, f)
		if funObjDecl == nil {
			return nil
		}

		funcDecl.Body = funObjDecl.Body
		funcDecl.Type = funObjDecl.Type
	case *ast.SelectorExpr:
		fd := findSelectorDeclaration(pass, f)
		if fd == nil {
			return nil
		}

		funcDecl.Body = fd.Body
		funcDecl.Type = fd.Type
	default:
		return nil
	}

	results := funcDecl.Type.Results.List
	if len(results) != 1 || !isExprHasType(pass, results[0].Type, testFuncType) {
		return nil
	}

	var funcs []ast.Expr

	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		if n == nil {
			return false
		}

		if retStmt, ok := n.(*ast.ReturnStmt); ok {
			if len(retStmt.Results) == 1 {
				funcs = append(funcs, retStmt.Results[0])
			}
		}

		return true
	})

	return funcs
}

// funcDefPosition returns a function's position.
// It works with anonymous functions as well with function names.
func funcDefPosition(pass *analysis.Pass, e ast.Expr) token.Pos {
	anonFunLit, ok := e.(*ast.FuncLit)
	if ok {
		return anonFunLit.Pos()
	}

	funIdent, ok := e.(*ast.Ident)
	if !ok {
		selExpr, ok := e.(*ast.SelectorExpr)
		if !ok {
			return token.NoPos
		}

		funIdent = selExpr.Sel
	}

	funDef, ok := pass.TypesInfo.Uses[funIdent]
	if !ok {
		return token.NoPos
	}

	return funDef.Pos()
}

// isSelectorCall checks is selExpr is a call expresion on specific callObj.
// Useful to check Run() call for t.Run or b.Run.
func isSelectorCall(pass *analysis.Pass, selExpr *ast.SelectorExpr, callObj types.Object) bool {
	sel, ok := pass.TypesInfo.Selections[selExpr]
	if !ok {
		return false
	}

	return sel.Obj() == callObj
}

// isExprHasType returns true if expr has expected type.
func isExprHasType(pass *analysis.Pass, expr ast.Expr, expType types.Type) bool {
	typeInfo, ok := pass.TypesInfo.Types[expr]
	if !ok {
		return false
	}

	return types.Identical(typeInfo.Type, expType)
}

// isIdentPackageName returns true if ident refers to the specified package.
func isIdentPackageName(pass *analysis.Pass, ident *ast.Ident, pkgName string) bool {
	obj := pass.TypesInfo.Uses[ident]
	if obj == nil {
		return false
	}

	pkgObj, ok := obj.(*types.PkgName)
	if !ok {
		return false
	}

	return pkgObj.Imported().Path() == pkgName
}

// findSelectorDeclaration returns function declaration called by selector expression.
func findSelectorDeclaration(pass *analysis.Pass, expr *ast.SelectorExpr) *ast.FuncDecl {
	xsel, ok := pass.TypesInfo.Selections[expr]
	if !ok {
		return nil
	}

	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if ok && fd.Recv != nil && len(fd.Recv.List) == 1 {
				recvType, ok := fd.Recv.List[0].Type.(*ast.Ident)
				if !ok {
					continue
				}

				recvObj, ok := pass.TypesInfo.Uses[recvType]
				if !ok {
					continue
				}

				if !(types.Identical(recvObj.Type(), xsel.Recv())) {
					continue
				}

				if fd.Name.Name == expr.Sel.Name {
					return fd
				}
			}
		}
	}

	return nil
}

// findFunctionDeclaration returns function declaration called by identity.
func findFunctionDeclaration(pass *analysis.Pass, ident *ast.Ident) *ast.FuncDecl {
	if ident.Obj != nil {
		if funObjDecl, ok := ident.Obj.Decl.(*ast.FuncDecl); ok {
			return funObjDecl
		}
	}

	obj := pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return nil
	}

	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			if funcDecl.Name.Pos() == obj.Pos() {
				return funcDecl
			}
		}
	}

	return nil
}

func findTypeObject(pass *analysis.Pass, typeName string) types.Object {
	parts := strings.Split(typeName, ".")
	pkgName := parts[0]
	typeName = parts[1]

	for _, pkg := range pass.Pkg.Imports() {
		if pkg.Name() != pkgName {
			continue
		}

		obj := pkg.Scope().Lookup(typeName)
		if obj != nil {
			return obj
		}
	}

	return nil
}
