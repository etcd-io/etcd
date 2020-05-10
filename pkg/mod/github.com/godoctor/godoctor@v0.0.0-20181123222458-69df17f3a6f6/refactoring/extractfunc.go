// Copyright 2015-2018 Auburn University and others. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package refactoring

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"sort"

	"github.com/godoctor/godoctor/analysis/cfg"
	"github.com/godoctor/godoctor/analysis/dataflow"
	"github.com/godoctor/godoctor/text"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/loader"
)

/* -=-=- Sorting -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

// type for sorting []*types.Var variables alphabetically by name
type varSlice []*types.Var

func (t varSlice) Len() int           { return len(t) }
func (t varSlice) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t varSlice) Less(i, j int) bool { return t[i].Name() < t[j].Name() }

func SortVars(vars []*types.Var) {
	sort.Sort(varSlice(vars))
}

/* -=-=- stmtRange -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

// stmtRange represents a sequence of consecutive statements in the body of a
// BlockStmt, CaseClause, or CommClause.
type stmtRange struct {
	// The sequence of ancestor nodes for the statement sequence from the
	// enclosing BlockStmt/CaseClause/CommClause upward through the root of
	// the AST.  pathToRoot[0] will be an instace of *ast.BlockStmt,
	// *ast.CaseClause, or *ast.CommClause, and
	// pathToRoot[len(pathToRoot)-1] will be an instance of *ast.File.
	pathToRoot []ast.Node
	// The start and ending indices (inclusive) of the first and last
	// statements if this sequence in the list of children for the
	// enclosing BlockStmt/CaseClause/CommClause.
	firstIdx, lastIdx int
	// Control flow graph for the enclosing function
	cfg *cfg.CFG
	// CFG blocks inside the selected statement range
	blocksInRange []ast.Stmt
	// For each block in the CFG, variables that are live at its entry
	liveIn map[ast.Stmt]map[*types.Var]struct{}
	// Definitions reaching the entrypoint to the selection
	defsReachingSelection map[ast.Stmt]struct{}
	// PackageInfo used to bind variable names to *types.Var objects
	pkgInfo *loader.PackageInfo
	// The package rooted func / method enclosing the selection
	enclosingFunc *ast.FuncDecl
}

// newStmtRange creates a stmtRange corresponding to a selected region of a
// file.  If the selected range of characters does not enclose complete
// statements, the stmtRange is adjusted (if possible) to the closest legal
// selection.  The given pkgInfo is used to determine the types and bindings of
// variables in the selection.
func newStmtRange(file *ast.File, start, end token.Pos, pkgInfo *loader.PackageInfo) (*stmtRange, error) {
	startPath, _ := astutil.PathEnclosingInterval(file, start, start)
	endPath, _ := astutil.PathEnclosingInterval(file, end-1, end-1)

	// Work downward from the root of the AST, counting the number of nodes
	// that enclose both the start and end of the selection
	deepestCommonAncestorDepth := -1
	for i := 0; i < min(len(startPath), len(endPath)); i++ {
		if startPath[len(startPath)-1-i] == endPath[len(endPath)-1-i] {
			deepestCommonAncestorDepth++
		} else {
			break
		}
	}

	// Find the depth of the deepest BlockStmt, CaseClause, or CommClause
	// enclosing both the start and end of the selection.  If the user
	// selected the initialization statement in an if-statement (or
	// something similar), raise an error; it cannot be extracted.
	blockDepth := deepestCommonAncestorDepth
	body := []ast.Stmt{}
loop:
	for blockDepth > 0 {
		switch node := startPath[len(startPath)-1-blockDepth].(type) {
		case *ast.BlockStmt:
			body = node.List
			break loop
		case *ast.CaseClause:
			body = node.Body
			break loop
		case *ast.CommClause:
			body = node.Body
			break loop
		case *ast.IfStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.ForStmt, *ast.RangeStmt: // removed *ast.CommClause
			if blockDepth != deepestCommonAncestorDepth {
				// We are inside one of these constructs, but
				// we haven't yet found an enclosing block/etc.
				return nil, errInvalidSelection("The initialization statement in an if, switch, type switch, for, or range statement cannot be extracted.")
			}
			blockDepth--
		default:
			blockDepth--
		}
	}

	if blockDepth <= 0 {
		return nil, errInvalidSelection("Please select a sequence of statements inside a block.")
	}

	// pathToRoot is the list of ancestor nodes common to all of the
	// statements in the selection, from the enclosing
	// BlockStmt/CaseClause/CommClause up through the root
	pathToRoot := startPath[len(startPath)-1-blockDepth:]

	var enclosingFunc *ast.FuncDecl
	for _, node := range pathToRoot {
		if f, ok := node.(*ast.FuncDecl); ok {
			enclosingFunc = f
			break
		}
	}
	if enclosingFunc == nil {
		return nil, errInvalidSelection("Please select a sequence of statements inside a function declaration.")
	}
	cfg := cfg.FromFunc(enclosingFunc)

	// Find the indices of the first and last statements whose positions
	// overlap the selection
	firstIdx := -1
	lastIdx := -1
	for i, stmt := range body {
		overlapStart := maxPos(start, stmt.Pos())
		overlapEnd := minPos(end, stmt.End())
		inSelection := overlapStart < overlapEnd
		if inSelection && firstIdx < 0 {
			// We found the first statement in the selection
			firstIdx = i
			lastIdx = i
		} else if inSelection && firstIdx >= 0 {
			// We found a subsequent statement in the selection
			lastIdx = i
		} else if !inSelection && lastIdx >= 0 {
			// We are beyond the end of the selection; no need to
			// check any more statements
			break
		}
	}

	if firstIdx < 0 || lastIdx < 0 {
		// There are no statements in the block.  Most likely, the user
		// selected an empty block, {}.
		return nil, errInvalidSelection("An empty block cannot be extracted")
	}

	liveIn, _ := dataflow.LiveVars(cfg, pkgInfo)

	result := &stmtRange{
		pathToRoot:            pathToRoot,
		firstIdx:              firstIdx,
		lastIdx:               lastIdx,
		cfg:                   cfg,
		blocksInRange:         nil,
		liveIn:                liveIn,
		defsReachingSelection: map[ast.Stmt]struct{}{},
		pkgInfo:               pkgInfo,
		enclosingFunc:         enclosingFunc,
	}

	// Determine the subset of blocks in the CFG that correspond to
	// statements within the selected region.
	blocksInRange := []ast.Stmt{}
	for _, stmt := range cfg.Blocks() {
		if result.Contains(stmt) {
			blocksInRange = append(blocksInRange, stmt)
		}
	}
	result.blocksInRange = blocksInRange

	// Find those definitions that reach the entry to the selected region.
	reaching := make(map[ast.Stmt]struct{})
	for _, entry := range result.EntryPoints() {
		for def := range dataflow.DefsReaching(entry, cfg, pkgInfo) {
			reaching[def] = struct{}{}
		}
	}
	result.defsReachingSelection = reaching

	return result, nil
}

// min returns the minimum of two integers.
func min(m, n int) int {
	if m < n {
		return m
	}
	return n
}

// minPos returns the minimum of two token positions
// (equivalently, the position that appears first)
func minPos(m, n token.Pos) token.Pos {
	if m < n {
		return m
	}
	return n
}

// maxPos returns the maximum of two token positions
// (equivalently, the position that appears last)
func maxPos(m, n token.Pos) token.Pos {
	if m > n {
		return m
	}
	return n
}

// selectedStmts returns the children of the enclosing
// BlockStmt/CaseClause/CommClause that comprise the selected region.  Note
// that this only includes immediate children; to visit nested statements, use
// Inspect.
func (r *stmtRange) selectedStmts() []ast.Stmt {
	list := []ast.Stmt{}
	switch node := r.pathToRoot[0].(type) {
	case *ast.BlockStmt:
		list = node.List
	case *ast.CaseClause:
		list = node.Body
	case *ast.CommClause:
		list = node.Body
	default:
		panic("unexpected node type")
	}
	return list[r.firstIdx : r.lastIdx+1]
}

// Inspect traverses the selected statements and their children.
func (r *stmtRange) Inspect(f func(ast.Node) bool) {
	for _, node := range r.selectedStmts() {
		ast.Inspect(node, f)
	}
}

// IsInAnonymousFunc returns true if the selected statements have at least one
// ancestor that is a FuncLit, i.e., an anonymous function.
func (r *stmtRange) IsInAnonymousFunc() bool {
	for _, node := range r.pathToRoot {
		if _, ok := node.(*ast.FuncLit); ok {
			return true
		}
	}
	return false
}

// ContainsAnonymousFunc returns true if a FuncLit node (i.e., an anonymous
// function) appears as a descendent of any of the selected statements.
func (r *stmtRange) ContainsAnonymousFunc() bool {
	flag := false
	r.Inspect(func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			flag = true
			return false
		}
		return true
	})
	return flag
}

// ContainsDefer returns true if any of the selected statements, or any of
// their desdendents, are defer statements (DeferStmt nodes).
func (r *stmtRange) ContainsDefer() bool {
	flag := false
	r.Inspect(func(n ast.Node) bool {
		if _, ok := n.(*ast.DeferStmt); ok {
			flag = true
			return false
		}
		return true
	})
	return flag
}

// ContainsReturn returns true if any of the selected statements, or any of
// their desdendents, are return statements (ReturnStmt nodes).
func (r *stmtRange) ContainsReturn() bool {
	flag := false
	r.Inspect(func(n ast.Node) bool {
		if _, ok := n.(*ast.ReturnStmt); ok {
			flag = true
			return false
		}
		return true
	})
	return flag
}

// Contains returns true if the given node lies (lexically) within the region
// of text corresponding to the selected statements.  Equivalently, it will
// return true if the given node is either a selected statement or a descendent
// of a selected statement.
func (r *stmtRange) Contains(node ast.Node) bool {
	stmts := r.selectedStmts()
	firstStmt := stmts[0]
	lastStmt := stmts[len(stmts)-1]
	return node.Pos() >= firstStmt.Pos() && node.End() <= lastStmt.End()
}

// Pos returns the starting position of the first statement in the selection.
func (r *stmtRange) Pos() token.Pos {
	return r.selectedStmts()[0].Pos()
}

// End returns the ending position (exclusive) of the last statement in the
// selection.
func (r *stmtRange) End() token.Pos {
	stmts := r.selectedStmts()
	return stmts[len(stmts)-1].End()
}

// EntryPoints returns the CFG block(s) corresponding to the statement(s)
// within the selected region that will be the first to execute, before any
// other statements in the selection.
func (r *stmtRange) EntryPoints() []ast.Stmt {
	entrySet := map[ast.Stmt]struct{}{}
	for _, b := range r.blocksInRange {
		for _, pred := range r.cfg.Preds(b) {
			if !r.Contains(pred) {
				entrySet[b] = struct{}{}
			}
		}
	}

	entryPoints := []ast.Stmt{}
	for b := range entrySet {
		entryPoints = append(entryPoints, b)
	}

	r.cfg.Sort(entryPoints)
	return entryPoints
}

// ExitDestinations returns the CFG block(s) corresponding to the statement(s)
// outside the selected region that could be the first to execute after the
// statements in the selection have executed.
func (r *stmtRange) ExitDestinations() []ast.Stmt {
	exitSet := map[ast.Stmt]struct{}{}
	for _, b := range r.blocksInRange {
		for _, succ := range r.cfg.Succs(b) {
			if !r.Contains(succ) {
				exitSet[succ] = struct{}{}
			}
		}
	}

	exitTo := []ast.Stmt{}
	for b := range exitSet {
		exitTo = append(exitTo, b)
	}

	r.cfg.Sort(exitTo)
	return exitTo
}

// LocalsLiveAtEntry returns the local variables that are live at the
// entrypoint(s) to the selected region.
func (r *stmtRange) LocalsLiveAtEntry() []*types.Var {
	entryPoints := r.EntryPoints()

	liveEntry := []*types.Var{}
	for _, entry := range entryPoints {
		for variable := range r.liveIn[entry] {
			liveEntry = append(liveEntry, variable)
		}
	}
	SortVars(liveEntry)
	return liveEntry
}

// LocalsLiveAfterExit returns the local variables that are live at the exit
// points from the selected region/at the entrypoints to the next statements
// after the selected statements have executed.
func (r *stmtRange) LocalsLiveAfterExit() []*types.Var {
	exitTo := r.ExitDestinations()

	liveExit := []*types.Var{}
	for _, exit := range exitTo {
		for variable := range r.liveIn[exit] {
			liveExit = append(liveExit, variable)
		}
	}
	SortVars(liveExit)
	return liveExit
}

// LocalsReferenced returns the local variables that are accessed by one or
// more of the selected statements.  It returns the variables that are
// (1) assigned, i.e., whose values are completely overwritten;
// (2) updated, i.e., a struct member or array element is modified;
// (3) declared via a var declaration or := operator;
// (4) used, i.e., whose values are read.
// Variables may appear in multiple sets.
func (r *stmtRange) LocalsReferenced() (asgt, updt, decl, use []*types.Var) {
	asgtSet, updtSet, declSet, useSet := dataflow.ReferencedVars(r.blocksInRange, r.pkgInfo)
	for v := range asgtSet {
		asgt = append(asgt, v)
	}
	for v := range updtSet {
		updt = append(updt, v)
	}
	for v := range declSet {
		decl = append(decl, v)
	}
	for v := range useSet {
		use = append(use, v)
	}
	SortVars(asgt)
	SortVars(decl)
	SortVars(use)
	return
}

func (r *stmtRange) String() string {
	stmts := r.selectedStmts()

	var b bytes.Buffer
	b.WriteString("Statement sequence from ")
	b.WriteString(reflect.TypeOf(stmts[0]).String())
	b.WriteString(" through ")
	b.WriteString(reflect.TypeOf(stmts[len(stmts)-1]).String())
	return b.String()
}

/* -=-=- extractedFunc -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-= */

// extractedFunc encapsulates information about the new function that will be
// created from the extracted code, along with how it should be called.
type extractedFunc struct {
	name       string                        // name of the new function
	recv       *types.Var                    // receiver variable, or nil
	params     []*types.Var                  // parameters for the new function
	returns    []*types.Var                  // variables whose values will be returned
	locals     []*types.Var                  // local variables to declare
	localInits map[*types.Var]ast.Expr       // initialization expressions for locals
	define     bool                          // x := f() instead of x = f()
	code       []byte                        // code to copy into the function body
	pkgFmt     func(p *types.Package) string // rewrite import uses
}

// SourceCode returns source code for (1) the new function declaration that
// should be inserted, and (2) the function call that should replace the
// selected statements.
func (f *extractedFunc) SourceCode() (funcDecl, funcCall string) {
	paramNames, paramTypes := namesAndTypes(f.params, f.pkgFmt)
	funcDeclParams := createParamDecls(paramNames, paramTypes)
	funcCallArgs := commaSeparated(paramNames)
	if f.recv != nil {
		recvType := types.TypeString(f.recv.Type(), f.pkgFmt)
		funcDecl = fmt.Sprintf("(%s %s) %s(%s)",
			f.recv.Name(), recvType, f.name, funcDeclParams)
		funcCall = fmt.Sprintf("%s.%s(%s)",
			f.recv.Name(), f.name, funcCallArgs)
	} else {
		funcDecl = fmt.Sprintf("%s(%s)", f.name, funcDeclParams)
		funcCall = fmt.Sprintf("%s(%s)", f.name, funcCallArgs)
	}

	names, types := namesAndTypes(f.locals, f.pkgFmt)
	localVarDecls := createVarDecls(names, types, initStrings(f.localInits))
	if len(f.returns) == 0 {
		funcDecl = fmt.Sprintf("\n\nfunc %s {\n%s%s\n}\n",
			funcDecl, localVarDecls, f.code)
		funcCall = fmt.Sprintf("%s", funcCall)
	} else {
		returnNames, returnTypes := namesAndTypes(f.returns, f.pkgFmt)
		returnExprs := commaSeparated(returnNames)
		returnStmt := "return " + returnExprs

		assignSymbol := " = "
		if f.define {
			assignSymbol = " := "
		}

		funcDefReturnTypes := commaSeparated(returnTypes)
		if len(returnNames) > 1 {
			funcDecl = fmt.Sprintf("\n\nfunc %s(%s) {\n%s%s\n%s\n}\n",
				funcDecl, funcDefReturnTypes, localVarDecls,
				f.code, returnStmt)
			funcCall = fmt.Sprintf("%s%s%s",
				returnExprs, assignSymbol, funcCall)
		} else {
			funcDecl = fmt.Sprintf("\n\nfunc %s %s {\n%s%s\n%s\n}\n",
				funcDecl, funcDefReturnTypes, localVarDecls,
				f.code, returnStmt)
			funcCall = fmt.Sprintf("%s%s%s",
				returnExprs, assignSymbol, funcCall)
		}
	}

	return funcDecl, funcCall
}

// namesAndTypes receives a list of variables and returns strings describing
// their names and types, suitable for use in variable declarations.
func namesAndTypes(vars []*types.Var, fmt types.Qualifier) (names []string, typez []string) {
	for _, a := range vars {
		if a.Name() != "_" {
			names = append(names, a.Name())
			typez = append(typez, types.TypeString(a.Type(), fmt))
		}
	}
	return
}

// initStrings receives a map from variables to (constant-valued) expressions
// and converts the keys and values to strings, returning a map from variable
// names to expression text.  If an expression is mapped to nil, then it is
// not included in the returned map.  This works because createVarDecls will
// declare the variable with a "var" declaration, which is exactly what we
// want in that situation.
func initStrings(inits map[*types.Var]ast.Expr) map[string]string {
	result := make(map[string]string)
	for variable, expr := range inits {
		if expr != nil {
			result[variable.Name()] = types.ExprString(expr)
		}
	}
	return result
}

// createVarDecls returns source code for a sequence of var statements
// declaring variables with the given names, types, and initial values.
func createVarDecls(names []string, types []string, localInits map[string]string) string {
	var buf bytes.Buffer
	for i := 0; i < len(names); i++ {
		if init, ok := localInits[names[i]]; ok {
			buf.WriteString(names[i] + " := " + init)
		} else {
			buf.WriteString("var " + names[i] + " " + types[i])
		}
		if i > 1 || i <= len(names)-1 {
			buf.WriteString("\n")
		}
	}
	return buf.String()
}

// commaSeparated concatenates the given strings, separating them by ", "
func commaSeparated(strings []string) string {
	var buf bytes.Buffer
	for k := 0; k < len(strings); k++ {
		buf.WriteString(strings[k])
		if k == len(strings)-1 {
			break
		}
		if k > 1 || k < len(strings)-1 {
			buf.WriteString(", ")
		}
	}
	return buf.String()
}

// createParamDecls returns source code for a parameter list, declaring
// function parameters with the given names and types.
func createParamDecls(names []string, types []string) string {
	var buf bytes.Buffer
	for k := 0; k < len(names); k++ {
		buf.WriteString(names[k] + " " + types[k])
		if k > 1 || k < len(names)-1 {
			buf.WriteString(", ")
		}
	}
	return buf.String()
}

/* -=-=- ExtractFunc -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

// The ExtractFunc refactoring is used to break down larger functions into
// smaller functions such that the logic of the code remains unchanged.
// The user is expected to extract recvTypeExpr part of code from the function and enter recvTypeExpr valid name
type ExtractFunc struct {
	RefactoringBase
	funcName  string     // name of the extracted function
	stmtRange *stmtRange // selected statements (to be extracted)
}

func (r *ExtractFunc) Description() *Description {
	return &Description{
		Name:      "Extract Function",
		Synopsis:  "Extracts statements to a new function/method",
		Usage:     "<new_name>",
		HTMLDoc:   extractFuncDoc,
		Multifile: false,
		Params: []Parameter{{
			Label:        "Name:",
			Prompt:       "Enter a name for the new function.",
			DefaultValue: "",
		}},
		OptionalParams: nil,
		Hidden:         false,
	}
}

func (r *ExtractFunc) Run(config *Config) *Result {
	if r.Init(config, r.Description()); r.Log.ContainsErrors() {
		return &r.Result
	}

	r.funcName = (config.Args[0]).(string)
	if !isIdentifierValid(r.funcName) {
		r.Log.Errorf("The name \"%s\" is not a valid Go identifier",
			r.funcName)
		return &r.Result
	}

	var err error
	r.stmtRange, err = newStmtRange(r.File, r.SelectionStart, r.SelectionEnd, r.SelectedNodePkg)
	if err != nil {
		r.Log.Error(err)
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return &r.Result
	}

	if r.stmtRange.IsInAnonymousFunc() {
		r.Log.Error("Code inside an anonymous function cannot be extracted.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return &r.Result
	}

	// Errors from here onward are non-fatal: The extraction can proceed,
	// but it may not preserve semantics.

	if r.stmtRange.ContainsAnonymousFunc() {
		r.Log.Error("Code containing anonymous functions may not extract correctly.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
	}

	if r.stmtRange.ContainsDefer() {
		r.Log.Error("Code containing defer statements may change behavior if it is extracted.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
	}

	if r.stmtRange.ContainsReturn() {
		r.Log.Error("Code containing return statements may change behavior if it is extracted.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
	}

	// The next two checks determine if the single-entry-single-exit
	// criterion is met.  The call to UpdateLog (below) will check the
	// refactored code for errors.  If the SESE criterion is not met,
	// that check will most likely point out the specific problems,
	// so don't make too much effort to describe them here.

	entryPoints := r.stmtRange.EntryPoints()
	if len(entryPoints) > 1 {
		r.Log.Error("There are multiple control flow paths into the selected statements.  Extraction will likely be incorrect.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
	}

	exitDests := r.stmtRange.ExitDestinations()
	if len(exitDests) > 1 {
		r.Log.Error("There are multiple control flow paths out of the selected statements.  Extraction will likely be incorrect.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
	}

	r.Log.ChangeInitialErrorsToWarnings()
	r.addEdits()
	r.FormatFileInEditor()
	r.UpdateLog(config, true) // Check for errors in the refactored code
	return &r.Result
}

// addEdits updates r.Edits, adding edits to insert a new function declaration
// and replace the selected statements with a call to that function.
func (r *ExtractFunc) addEdits() {
	funcDecl, funcCall := r.createExtractedFunc().SourceCode()

	// Replace the selected statements with a function call
	offset := r.Program.Fset.Position(r.stmtRange.Pos()).Offset
	length := r.Program.Fset.Position(r.stmtRange.End()).Offset - offset
	r.Edits[r.Filename].Add(&text.Extent{offset, length}, funcCall)

	next := r.Program.Fset.Position(r.stmtRange.enclosingFunc.End()).Offset

	// Insert the new function declaration
	r.Edits[r.Filename].Add(&text.Extent{next, 0}, funcDecl)
}

// createExtractedFunc returns an extractedFunc, which contains information
// about the extracted function and how it should be called.  Source code can
// be obtained from the extractedFunc object.
func (r *ExtractFunc) createExtractedFunc() *extractedFunc {
	recv, params, returns, locals, localInits, declareResult := r.analyzeVars()

	startOffset := r.Program.Fset.Position(r.stmtRange.Pos()).Offset
	endOffset := r.Program.Fset.Position(r.stmtRange.End()).Offset
	code := r.FileContents[startOffset:endOffset]

	return &extractedFunc{
		name:       r.funcName,
		recv:       recv,
		params:     params,
		returns:    returns,
		locals:     locals,
		localInits: localInits,
		define:     declareResult,
		code:       code,
		pkgFmt:     pkgUseFmt(r.SelectedNodePkg.Pkg),
	}
}

// pkgUseFmt returns a types.Qualifier similar to types.RelativeTo,
// but instead of returning full paths, it returns only the package's base
// name, i.e. 'github.com/some/pkg' -> 'pkg'. The current package's name
// is also omitted (since it would be a circular dependency on itself).
func pkgUseFmt(pkg *types.Package) types.Qualifier {
	if pkg == nil {
		return nil // wat
	}
	return func(other *types.Package) string {
		if pkg == other {
			return "" // same package; unqualified
		}
		return other.Name()
	}
}

// analyzeVars determines (1) whether the extracted function should be a method
// and if so, what its receiver should be; (2) which local variables used in
// the selected statements should be passed as arguments to the extracted
// function; (3) which local variables' values must be returned from the
// extracted function; (4) which local variables can be redeclared in the
// extracted function (i.e., they do not need to be passed as arguments); and
// (5) when the selected statements are replaced with a function call, whether
// the call should have the form x := f() or x = f() -- i.e., whether the
// result variables should be declared or simply assigned.
func (r *ExtractFunc) analyzeVars() (recv *types.Var,
	params, returns, locals []*types.Var,
	localInits map[*types.Var]ast.Expr,
	declareResult bool) {

	aliveFirst := r.stmtRange.LocalsLiveAtEntry()
	aliveLast := r.stmtRange.LocalsLiveAfterExit()
	assigned, updated, declared, used := r.stmtRange.LocalsReferenced()
	defined := union(union(assigned, updated), declared)

	// Params = LIVE_IN[Entry(selectionnode)] ⋂ USE[selection]
	params = intersection(aliveFirst, union(union(used, assigned), updated))

	// returns = LIVE_OUT[exit(sel)] ⋂ DEF[sel]
	// If someStruct is a pointer and someStruct.field is assigned, but
	// someStruct itself is never reassigned, then it does not need to be
	// returned.  Likewise, if individual elements of a slice are updated
	// but the slice itself is not reassigned, then the slice variable
	// does not need to be returned.
	updatedOnlyThruPointers := difference(r.varsWithPointerOrSliceTypes(updated), assigned)
	returns = difference(
		intersection(aliveLast, defined),
		updatedOnlyThruPointers)

	locals = difference(
		union(difference(assigned, params),
			difference(used, aliveFirst)),
		declared)

	// If we are returning the value of a variable declared in the
	// selected statements, then the result variable needs to be declared.
	declareResult = len(intersection(returns, declared)) > 0

	if recvNode := r.stmtRange.enclosingFunc.Recv; recvNode != nil {
		recv = r.SelectedNodePkg.ObjectOf(recvNode.List[0].Names[0]).(*types.Var)
		params = difference(params, []*types.Var{recv})
		returns = difference(returns, []*types.Var{recv})
		locals = difference(locals, []*types.Var{recv})
	}

	// If an argument always has a constant value, there is no reason to
	// pass it as an argument.  Instead, make it a local variable, and
	// set it equal to its constant value.
	constants := r.constantValues(params)
	for param := range constants {
		params = difference(params, []*types.Var{param})
		locals = append(locals, param)
	}

	namedReturns := make(map[*types.Var]struct{})

	if r.stmtRange.enclosingFunc.Type.Results != nil {
		for _, field := range r.stmtRange.enclosingFunc.Type.Results.List {
			// this won't be necessary for unnamed returns
			if len(field.Names) == 0 {
				continue
			}

			namedReturns[r.stmtRange.pkgInfo.ObjectOf(field.Names[0]).(*types.Var)] = struct{}{}
		}
	}

	// figure out which locals are actually just named
	// returns and add them to the set of return vars
	for _, v := range locals {
		if _, exists := namedReturns[v]; exists {
			returns = append(returns, v)
		}
	}

	// Sort each set of variables so we always extract in the same order.
	SortVars(params)
	SortVars(returns)
	SortVars(locals)
	return recv, params, returns, locals, constants, declareResult
}

// defs takes a list of variables and determines which are constant-valued; it
// returns a map from (a subset of those) variables to the expressions defining
// their constant values.  If the value expression is nil, then the variable
// is defined by a "var" declaration with no initialization expression.
//
// The analysis is not too sophisticated.  We only say the variable is
// constant-valued if
// (1) exactly one definition reaches the entry to the selected region, and
// (2) that definition has one of the following forms:
//     var name type
//     var name type = value
//     name := value
//     name = value
func (r *ExtractFunc) constantValues(varList []*types.Var) map[*types.Var]ast.Expr {
	result := make(map[*types.Var]ast.Expr)
	for variable, defs := range r.defsInitializing(varList) {
		// fmt.Println(variable.Name(), " has ", len(defs), " defs")
		// for s, _ := range defs {
		// 	fmt.Printf("Line %d: ",
		// 		r.Program.Fset.Position(s.Pos()).Line,
		// 		astutil.NodeDescription(s))
		// }
		if def, ok := extractSingleton(defs); ok {
			if expr, isConstant := r.constantAssigned(def); isConstant {
				result[variable] = expr
			}
		}
	}
	return result
}

// defsInitializing takes a list of variables and returns a map from each
// variable to the set of statements that assign that variable's value at the
// entry to inside the selection.
//
// This is used to determine which variables are constant-valued.
func (r *ExtractFunc) defsInitializing(varList []*types.Var) map[*types.Var]map[ast.Stmt]struct{} {
	result := make(map[*types.Var]map[ast.Stmt]struct{})

	// When the first statement in the selection is a for-loop, a definition
	// inside the loop may reach the beginning of the selection.  However,
	// these definitions do not affect the initial value of variables, so we
	// exclude them.
	excluded := make(map[ast.Stmt]bool)
	for _, stmt := range r.stmtRange.EntryPoints() {
		if isForOrRangeStmt(stmt) {
			ast.Inspect(stmt, func(n ast.Node) bool {
				if s, ok := n.(ast.Stmt); ok {
					excluded[s] = true
				}
				return true
			})
		}
	}

	// Make sure every variable has an entry in the result map
	for _, variable := range varList {
		result[variable] = make(map[ast.Stmt]struct{})
	}

	// Add all definitions to the result map
	for stmt := range r.stmtRange.defsReachingSelection {
		if excluded[stmt] {
			continue
		}

		asgtSet, updtSet, declSet, _ := dataflow.ReferencedVars(
			[]ast.Stmt{stmt}, r.stmtRange.pkgInfo)
		for variable := range asgtSet {
			if _, found := result[variable]; found {
				result[variable][stmt] = struct{}{}
			}
		}
		for variable := range updtSet {
			if _, found := result[variable]; found {
				result[variable][stmt] = struct{}{}
			}
		}
		for variable := range declSet {
			if _, found := result[variable]; found {
				result[variable][stmt] = struct{}{}
			}
		}
	}

	return result
}

func isForOrRangeStmt(stmt ast.Stmt) bool {
	switch stmt.(type) {
	case *ast.ForStmt, *ast.RangeStmt:
		return true
	default:
		return false
	}
}

// extractSingleton returns the only element in the given set, returning that
// element and true if it is a singleton set; otherwise, it returns nil and
// false.
func extractSingleton(set map[ast.Stmt]struct{}) (ast.Stmt, bool) {
	if len(set) != 1 {
		return nil, false
	}
	for stmt := range set {
		return stmt, true
	}
	panic("Unreachable")
}

// constantAssigned determines whether the given statement assigns a constant
// value to an identifier, returning the constant expression and true if so.
func (r *ExtractFunc) constantAssigned(stmt ast.Stmt) (ast.Expr, bool) {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		if len(s.Lhs) == 1 && len(s.Rhs) == 1 &&
			isIdentifier(s.Lhs[0]) && isConstant(s.Rhs[0]) {
			// identifier = expr
			return s.Rhs[0], true
		}
		return nil, false

	case *ast.DeclStmt:
		if decl, ok := s.Decl.(*ast.GenDecl); ok && len(decl.Specs) == 1 {
			if valueSpec, ok := decl.Specs[0].(*ast.ValueSpec); ok {
				if len(valueSpec.Names) == 1 &&
					isIdentifier(valueSpec.Names[0]) {
					if len(valueSpec.Values) == 0 {
						// var name type
						return nil, true
					}
					if len(valueSpec.Values) == 1 &&
						isConstant(valueSpec.Values[0]) {
						// name := value
						return valueSpec.Values[0], true
					}
				}
			}
		}
		return nil, false

	default:
		return nil, false
	}
}

func isIdentifier(expr ast.Expr) bool {
	_, ok := expr.(*ast.Ident)
	return ok
}

// constantIds are identifiers that are considered to be constants.
// See analyzeVars and constantValues.
//
// We omit "nil" to avoid introducing "use of untyped nil" errors.  To work
// around this, we would need to always introduce a var declaration for nil
// rather than using the short assignment operator).
var constantIds map[string]bool = map[string]bool{
	"false": true,
	"true":  true,
}

// isConstant returns true if an expression is considered to be a constant.
// See analyzeVars and constantValues.
//
// For our purposes, constants are BasicLits (integer, float, imaginary,
// character, and string literals) and certain identifiers (true and false).
func isConstant(expr ast.Expr) bool {
	switch x := expr.(type) {
	case *ast.BasicLit:
		return true
	case *ast.Ident:
		return constantIds[x.String()]
	default:
		return false
	}
}

// varsWithPointerOrSliceTypes receives a list of variables and returns those
// whose type is either a pointer or slice type.
func (r *ExtractFunc) varsWithPointerOrSliceTypes(varList []*types.Var) []*types.Var {
	result := []*types.Var{}
	for _, a := range varList {
		switch a.Type().(type) {
		case *types.Pointer, *types.Slice:
			result = append(result, a)
		}
	}
	return result
}

func intersection(s1, s2 []*types.Var) []*types.Var {
	result := []*types.Var{}
	for i := 0; i < len(s2); i++ {
		for j := 0; j < len(s1); j++ {
			if s2[i] == s1[j] {
				result = append(result, s2[i])
			}
		}
	}
	return result
}

func union(v1, v2 []*types.Var) []*types.Var {
	insec := intersection(v1, v2) // check for duplicates and removes them
	for _, a := range v2 {
		v1 = append(v1, a)
	}
	v1 = difference(v1, insec)
	for _, b := range insec {
		v1 = append(v1, b) // adding back the variables to the array only once
	}
	SortVars(v1)
	return v1
}

func difference(use, in []*types.Var) []*types.Var {
	var flag bool
	var result []*types.Var
	for i := 0; i < len(use); i++ {
		flag = false
		for j := 0; j < len(in); j++ {
			if use[i].Name() == in[j].Name() || use[i].Name() == "_" {
				flag = true
				break
			}
		}
		if flag == false {
			result = append(result, use[i])
		}
	}
	return result
}

const extractFuncDoc = `
  <h4>Purpose</h4>
  <p>The Extract Function refactoring creates a new function (or method) from a
  sequence of statements, then replaces the original statements with a call to
  that function.</p>

  <h4>Usage</h4>
  <ol class="enum">
    <li>Select a sequence of one or more statements inside an existing function
        or method.</li>
    <li>Activate the Extract Function refactoring.</li>
    <li>Enter a name for the new function that will be created.</li>
  </ol>

  <p>The refactoring will automatically determine what local variables need to
  be passed to the extracted function and returned as results.</p>

  <p>An error or warning will be reported if the selected statements cannot be
  extracted into a new function.  Usually, this occurs because they contain a
  statement like <tt>return</tt> which will have a different meaning in the
  extracted function.</p>

  <h4>Limitations</h4>
  <ul>
    <li>Code containing <tt>return</tt> statements, <tt>defer</tt> statements,
    or anonymous functions cannot be extracted.</li>
  </ul>
`
