// Copyright 2015-2017 Auburn University and others. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package refactoring

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"

	"github.com/godoctor/godoctor/analysis/cfg"
	"github.com/godoctor/godoctor/analysis/dataflow"
	"github.com/godoctor/godoctor/text"
)

type ExtractLocal struct {
	RefactoringBase
	varName string
}

func (r *ExtractLocal) Description() *Description {
	return &Description{
		Name:      "Extract Local Variable",
		Synopsis:  "Extracts an expression, assigning it to a variable",
		Usage:     "<new_name>",
		HTMLDoc:   extractLocalDoc,
		Multifile: false,
		Params: []Parameter{{
			Label:        "Name: ",
			Prompt:       "Enter a name for the new variable.",
			DefaultValue: "",
		}},
		OptionalParams: nil,
		Hidden:         false,
	}
}

func (r *ExtractLocal) Run(config *Config) *Result {
	r.Init(config, r.Description())
	r.Log.ChangeInitialErrorsToWarnings()
	if r.Log.ContainsErrors() {
		return &r.Result
	}

	r.varName = config.Args[0].(string)
	if !isIdentifierValid(r.varName) {
		r.Log.Errorf("The name \"%s\" is not a valid Go identifier",
			r.varName)
		return &r.Result
	}

	// First check preconditions that cause fatal errors
	// (i.e., the transformation cannot proceed unless they are met,
	// since it won't know where to insert the extracted expression,
	// or the extraction is likely to produce invalid code)
	if r.checkSelectedNodeIsExpr() &&
		r.checkExprHasValidType() &&
		r.checkExprIsNotFieldSelector() &&
		r.checkExprAddressIsNotTaken() &&
		r.checkExprIsNotInTypeNode() &&
		r.checkExprIsNotKeyInKeyValueExpr() &&
		r.checkExprIsNotFunctionInCallExpr() &&
		r.checkExprIsNotInTypeAssertionType() &&
		r.checkExprHasEnclosingStmt() &&
		r.checkEnclosingStmtIsAllowed() &&
		r.checkExprIsNotAssignStmtLhs() &&
		r.checkEnclosingIfStmt() &&
		r.checkEnclosingForStmt() &&
		r.checkExprIsNotRangeStmtLhs() &&
		r.checkExprIsNotInCaseClauseOfTypeSwitchStmt() {
		// Now, check preconditions that are only for semantic
		// preservation (i.e., they should not block the refactoring,
		// but the user should be made aware of a potential problem)
		r.checkForNameConflict()
		// Finally, perform the transformation
		r.addEdits(r.findStmtToInsertBefore())
		r.FormatFileInEditor()
		r.UpdateLog(config, false)
	}
	return &r.Result
}

// checkSelectedNodeIsExpr checks that the user has selected an expression,
// logging an error and returning false iff not.
//
// If this function returns true, r.SelectedNode.(ast.Expr) can be asserted.
func (r *ExtractLocal) checkSelectedNodeIsExpr() bool {
	if r.SelectedNode == nil {
		r.Log.Error("Please select an expression to extract.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}

	if _, ok := r.SelectedNode.(ast.Expr); !ok {
		r.Log.Error("Please select an expression to extract.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		r.Log.Errorf("(Selected node: %s)", reflect.TypeOf(r.SelectedNode))
		r.Log.AssociatePos(r.SelectedNode.Pos(), r.SelectedNode.Pos())
		return false
	}
	return true
}

// checkExprHasValidType determines the type of the selected expression and
// determines whether it can be assigned to a variable, logging an error and
// returning false if it cannot.
func (r *ExtractLocal) checkExprHasValidType() bool {
	exprType := r.SelectedNodePkg.TypeOf(r.SelectedNode.(ast.Expr))
	// fmt.Printf("Node is %s\n", reflect.TypeOf(r.SelectedNode))
	// fmt.Printf("Type is %s\n", exprType)

	if _, isFunctionType := exprType.(*types.Tuple); isFunctionType {
		r.Log.Errorf("The selected expression cannot be assigned to a variable since it has a tuple type %s", exprType)
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}

	if basic, isBasic := exprType.(*types.Basic); isBasic && (basic.Kind() == types.Invalid || basic.Info() == types.IsUntyped) {
		r.Log.Error("The selected expression cannot be assigned to a variable.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}

	return true
}

func (r *ExtractLocal) checkExprIsNotFieldSelector() bool {
	parentNode := r.PathEnclosingSelection[1]
	if selectorExpr, ok := parentNode.(*ast.SelectorExpr); ok {
		if selectorExpr.Sel == r.SelectedNode {
			r.Log.Error("A field selector cannot be extracted.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}
	return true
}

func (r *ExtractLocal) checkExprAddressIsNotTaken() bool {
	// This isn't completely correct, since &((((x)))) also takes an
	// address, but it's close enough for now
	parentNode := r.PathEnclosingSelection[1]
	if unary, ok := parentNode.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		r.Log.Error("An expression cannot be extracted if its address is taken.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}
	return true
}

func (r *ExtractLocal) checkExprIsNotInTypeNode() bool {
	for _, node := range r.PathEnclosingSelection {
		if isTypeNode(node) {
			r.Log.Error("An expression used to specify a type cannot be extracted.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}
	return true
}

// isTypeNode returns true iff the given AST node is an ArrayType,
// InterfaceType, MapType, StructType, or TypeSpec node.
func isTypeNode(node ast.Node) bool {
	switch node.(type) {
	case *ast.ArrayType:
		return true
	case *ast.InterfaceType:
		return true
	case *ast.MapType:
		return true
	case *ast.StructType:
		return true
	case *ast.TypeSpec:
		return true
	default:
		return false
	}
}

func (r *ExtractLocal) checkExprIsNotKeyInKeyValueExpr() bool {
	for i, node := range r.PathEnclosingSelection {
		if kv, ok := node.(*ast.KeyValueExpr); ok && i > 0 && r.PathEnclosingSelection[i-1] == kv.Key {
			r.Log.Error("The key in a key-value expression cannot be extracted.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}
	return true
}

func (r *ExtractLocal) checkExprIsNotInTypeAssertionType() bool {
	for i, node := range r.PathEnclosingSelection {
		if ta, ok := node.(*ast.TypeAssertExpr); ok && i > 0 && r.PathEnclosingSelection[i-1] == ta.Type {
			r.Log.Error("The selected expression cannot be extracted since it is part of the type in a type assertion.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}
	return true
}

func (r *ExtractLocal) checkExprIsNotFunctionInCallExpr() bool {
	parent := r.PathEnclosingSelection[1]
	if call, ok := parent.(*ast.CallExpr); ok {
		if r.SelectedNode == call.Fun {
			r.Log.Error("The function name in a function call expression cannot be extracted.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}
	return true
}

// enclosingStmtIndex returns the index into r.PathEnclosingSelection of the
// smallest ast.Stmt enclosing the selection, or -1 if the selection is not
// in a statement.
//
// If this returns a nonnegative value,
// r.PathEnclosingSelection[r.enclosingStmtIndex()].(ast.Stmt)
// can be asserted.
//
// See enclosingStmt
func (r *ExtractLocal) enclosingStmtIndex() int {
	for i, node := range r.PathEnclosingSelection {
		if _, ok := node.(ast.Stmt); ok {
			return i
		}
	}
	return -1
}

// enclosingStmt returns the smallest ast.Stmt enclosing the selection.
//
// Precondition: r.enclosingStmtIndex() >= 0
func (r *ExtractLocal) enclosingStmt() ast.Stmt {
	return r.PathEnclosingSelection[r.enclosingStmtIndex()].(ast.Stmt)
}

func (r *ExtractLocal) checkExprHasEnclosingStmt() bool {
	if r.enclosingStmtIndex() < 0 {
		r.Log.Error("The selected expression cannot be extracted " +
			"since it is not in an executable statement.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}
	return true
}

// checkEnclosingStmtIsAllowed looks at the statement in which the selected
// expression appears and determines if the expression can be extracted,
// logging an error and returning false if it cannot.
//
// Precondition: r.enclosingStmtIndex() >= 0
func (r *ExtractLocal) checkEnclosingStmtIsAllowed() bool {
	// fmt.Printf("Enclosing stmt is %s\n", reflect.TypeOf(r.enclosingStmt()))
	switch r.enclosingStmt().(type) {
	case *ast.AssignStmt:
		return true
	case *ast.CaseClause:
		return true
	//case *ast.DeclStmt: // const, type, or var
	//case *ast.DeferStmt:
	//case *ast.EmptyStmt: impossible
	case *ast.ExprStmt:
		return true
	case *ast.ForStmt:
		return true
	//case *ast.GoStmt:
	case *ast.IfStmt:
		return true
	//case *ast.IncDecStmt:
	//case *ast.LabeledStmt not allowed - label cannot be extracted
	case *ast.RangeStmt:
		return true
	case *ast.ReturnStmt:
		return true
	//case *ast.SelectStmt:
	//case *ast.SendStmt:
	//case *ast.SwitchStmt:
	//case *ast.TypeSwitchStmt:
	default:
		r.Log.Errorf("The selected expression cannot be extracted.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		r.Log.Errorf("(Enclosing statement is %s)", reflect.TypeOf(r.enclosingStmt()))
		r.Log.AssociatePos(r.enclosingStmt().Pos(), r.enclosingStmt().Pos())
		return false
	}
}

// checkExprIsNotAssignStmtLhs determines if the selected node is one of the
// LHS expressions for the given assignment statement, logging an error and
// returning false if it is.
//
// Note, in particular, that this prevents extracting _.
//
// Note that it is acceptable to extract a subexpression of an LHS expression
// (e.g., the subscript expression in a[i+2]=...), but not the entire expression.
func (r *ExtractLocal) checkExprIsNotAssignStmtLhs() bool {
	for _, node := range r.PathEnclosingSelection {
		if asgt, ok := node.(*ast.AssignStmt); ok {
			for _, lhsExpr := range asgt.Lhs {
				if r.SelectedNode == lhsExpr {
					r.Log.Error("The selected expression cannot be extracted since it is assigned to.")
					r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
					return false
				}
			}
		}
	}
	return true
}

// checkEnclosingIfStmt determines if the selected expression is in the header
// of an if-statement (or nested else-if), and then performs a reaching
// definitions analysis to determine if it is safe to extract the selected
// expression, logging an error if it is not.
//
// There are several problems with if-statements:
// (1) if x := 3; x < 5
//     Here, the definition (and declaration) of x in the init statement
//     reaches the condition, so an expression involving x cannot be
//     extracted from the condition.
// (2) if thing, ok := x.(*Thing); ok
//     The type assertion cannot be extracted, since it assigns both thing and
//     ok in this context.
// (3) if value, found := myMap[entry]
//     Similar.
// (4) if x := 3; x < 0 {} else if x < 5 {}
//     The "x < 5" depends on the initialization of the parent if-statement.
//
// Precondition: r.enclosingStmtIndex() >= 0
func (r *ExtractLocal) checkEnclosingIfStmt() bool {
	var isInInit, isInCond bool
	var ifStmt *ast.IfStmt
	ifStmt, isInInit = r.isIfStmtInit()
	if !isInInit {
		ifStmt, isInCond = r.enclosingStmt().(*ast.IfStmt)
		if !isInCond {
			// Not in the header of an if-statement; OK to extract
			return true
		}
	}

	du := r.defUse()

	if isInCond && !r.checkForReachingDefsFromIfStmtInit(ifStmt, du) {
		return false
	}

	ifStmtIdx := r.enclosingStmtIndex()
	if isInInit {
		ifStmtIdx++
	}
	return r.checkForReachingDefsFromElseIfChain(ifStmtIdx, du)
}

// isIfStmtInit determines if the selected expression is part of an
// if-statement's initialization, returning true or false; if it returns true,
// the second return value is the enclosing if-statement.
func (r *ExtractLocal) isIfStmtInit() (*ast.IfStmt, bool) {
	index := r.enclosingStmtIndex()
	if ifStmt, found := r.PathEnclosingSelection[index+1].(*ast.IfStmt); found {
		return ifStmt, r.enclosingStmt() == ifStmt.Init
	}
	return nil, false
}

// defUse performs a reaching definitions analysis on the function enclosing
// the selected expression, returning the result of the analysis.
// See varsInSelectionWithReachingDefsFrom.
func (r *ExtractLocal) defUse() map[ast.Stmt]map[ast.Stmt]struct{} {
	cfg := cfg.FromFunc(r.enclosingFuncDecl()) // We must be in a function
	defUse := dataflow.DefUse(cfg, r.SelectedNodePkg)
	// dataflow.PrintDefUse(os.Stderr, r.Program.Fset, r.SelectedNodePkg, du)
	return defUse
}

// enclosingFuncDecl returns the smallest ast.FuncDecl enclosing the selection,
// or nil if the selection is not in a function.
func (r *ExtractLocal) enclosingFuncDecl() *ast.FuncDecl {
	for _, node := range r.PathEnclosingSelection {
		if funcDecl, ok := node.(*ast.FuncDecl); ok {
			return funcDecl
		}
	}
	return nil
}

// checkForReachingDefsFromIfStmtInit returns true iff a variable assignment
// in the given if-statement's initialization statement reaches a use in its
// condition.
//
// For example:
//     if variable := 1; variable < 5 {}  // Variable cannot be extracted
func (r *ExtractLocal) checkForReachingDefsFromIfStmtInit(ifStmt *ast.IfStmt, du map[ast.Stmt]map[ast.Stmt]struct{}) bool {
	if ifStmt.Init == nil {
		return true
	}

	vars := r.varsInSelectionWithReachingDefsFrom(ifStmt.Init, du)
	if len(vars) > 0 {
		r.Log.Errorf("This expression cannot be extracted "+
			"because it uses %s assigned in the "+
			"enclosing if-statement's initialization "+
			"statement.", describeVars(vars))
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}

	return true
}

// reachingVarsFrom returns the set of variables assigned in "from" that are
// used in the selected expression
func (r *ExtractLocal) varsInSelectionWithReachingDefsFrom(from ast.Stmt, defUse map[ast.Stmt]map[ast.Stmt]struct{}) map[*types.Var]struct{} {
	result := map[*types.Var]struct{}{}

	if _, found := defUse[from][r.enclosingStmt()]; !found {
		return result
	}

	asgt, updt, decl, _ := dataflow.ReferencedVars([]ast.Stmt{from},
		r.SelectedNodePkg)
	use := dataflow.Vars(r.SelectedNode.(ast.Expr), r.SelectedNodePkg)

	for variable := range asgt {
		if _, used := use[variable]; used {
			result[variable] = struct{}{}
		}
	}
	for variable := range updt {
		if _, used := use[variable]; used {
			result[variable] = struct{}{}
		}
	}
	for variable := range decl {
		if _, used := use[variable]; used {
			result[variable] = struct{}{}
		}
	}
	return result
}

// describeVars receives a set of variables (e.g., x, y, and z) and returns
// the string "the variables x, y, and z" (for use in an error message)
func describeVars(vars map[*types.Var]struct{}) string {
	names := []string{}
	for variable := range vars {
		names = append(names, variable.Name())
	}

	var b bytes.Buffer
	b.WriteString("the variable")
	if len(vars) > 1 {
		b.WriteString("s")
	}

	for i := 0; i < len(names); i++ {
		if i == 0 {
			b.WriteString(" ")
		} else if i < len(names)-1 {
			b.WriteString(", ")
		} else {
			b.WriteString(", and ")
		}
		b.WriteString(names[i])
	}
	return b.String()
}

// checkForReachingDefsFromElseIfChain determines if the selected expression
// occurs in an if-statement that is the "else if" clause of another
// if-statement.  If it is, it follows the chain of else-if's upward to the
// initial if-statement.  If any of those if-statements define a variable that
// is used in the extracted expression, it raises an error.
//
// The extracted variable will be inserted above the first statement in the
// else-if chain, so it must not depend on any variables assigned in those
// statements.
//
// For example:
//     if a := 1; false {
//     } else if b := 2; false {
//     } else if c := 3; a + b == c {  // a + b cannot be extracted because
//                                     // the extracted variable assignment
//                                     // will be inserted before the outermost
//                                     // if-statement
//     }
//
// Precondition: The selected expression occurs in either the initialization
// statement or the condition of an if-statement.
//
// The ifStmtIdx argument is the index in r.PathEnclosingSelection of the
// nearest IfStmt node enclosing the selected expression.
func (r *ExtractLocal) checkForReachingDefsFromElseIfChain(ifStmtIdx int, du map[ast.Stmt]map[ast.Stmt]struct{}) bool {
	// Find the index of the beginning of the else-if chain
	last := r.findBeginningOfElseIfChain(ifStmtIdx)

	// Skip the nearest enclosing if-statement; we have already checked for
	// definitions reaching its condition from its initialization.

	// Start from the next if-statement: we are its else-if statement.
	// Work upward to successive enclosing if-statements, checking for
	// definitions that reach the selected expression.
	for i := ifStmtIdx + 1; i <= last; i++ {
		ifStmt := r.PathEnclosingSelection[i].(*ast.IfStmt)
		vars := r.varsInSelectionWithReachingDefsFrom(ifStmt.Init, du)
		if len(vars) > 0 {
			line := r.Program.Fset.Position(ifStmt.Pos()).Line
			r.Log.Errorf("This expression cannot be extracted "+
				"because it uses %s assigned in the enclosing "+
				"if-statement on line %d.",
				describeVars(vars), line)
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}
	return true
}

// checkEnclosingForStmt determines if the selected expression is in the header
// of a for-statement, and then performs a reaching definitions analysis to
// determine if it is safe to extract the selected expression, logging an error
// if it is not.
//
// There are several problems with if-statements:
// (1) for i = 0; i < x; i++ {}
//     The variable i cannot be extracted from the condition because it depends
//     on the definition (assignment) in the initialization.  However, the
//     variable x can be extracted.
// (2) for i = 0; i < x; i++ { x-- }
//     Here, the variable x cannot be extracted from the condition because it
//     is assigned in the body of the loop.
//
// Precondition: r.enclosingStmtIndex() >= 0
func (r *ExtractLocal) checkEnclosingForStmt() bool {
	var isInInit, isInCond, isInPost bool
	var enclosingForStmt *ast.ForStmt
	enclosingForStmt, isInInit = r.isForStmtInit()
	if !isInInit {
		enclosingForStmt, isInCond = r.enclosingStmt().(*ast.ForStmt)
		if !isInCond {
			enclosingForStmt, isInPost = r.isForStmtPost()
			if !isInPost {
				return true
			}
		}
	}

	if isInInit {
		return true
	}

	if isInPost {
		r.Log.Error("Expressions cannot be extracted from a " +
			"for-statement's post-iteration statement.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}

	return r.checkForStmtReachingDefs(enclosingForStmt)
}

// isForStmtInit determines if the selected expression is part of a
// for-statement's initialization statement, returning true or false; if it
// returns true, the second return value is the enclosing for-statement.
func (r *ExtractLocal) isForStmtInit() (*ast.ForStmt, bool) {
	index := r.enclosingStmtIndex()
	if forStmt, found := r.PathEnclosingSelection[index+1].(*ast.ForStmt); found {
		return forStmt, r.enclosingStmt() == forStmt.Init
	}
	return nil, false
}

// isForStmtPost determines if the selected expression is part of a
// for-statement's post-iteration statement, returning true or false; if it
// returns true, the second return value is the enclosing for-statement.
func (r *ExtractLocal) isForStmtPost() (*ast.ForStmt, bool) {
	index := r.enclosingStmtIndex()
	if forStmt, found := r.PathEnclosingSelection[index+1].(*ast.ForStmt); found {
		return forStmt, r.enclosingStmt() == forStmt.Post
	}
	return nil, false
}

// checkForStmtReachingDefs finishes the precondition checking from
// checkEnclosingForStmt, performing a reaching definitions analysis to
// identify variable assignments in the given for-loop that may make it
// illegal to extract the selected expression.
//
// Precondition: The selected expression is a subexpression of the given
// for-loop's condition expression.
func (r *ExtractLocal) checkForStmtReachingDefs(enclosingForStmt *ast.ForStmt) bool {
	defUse := r.defUse()

	if enclosingForStmt.Init != nil {
		vars := r.varsInSelectionWithReachingDefsFrom(
			enclosingForStmt.Init, defUse)
		if len(vars) > 0 {
			r.Log.Errorf("This expression cannot be extracted "+
				"because it uses %s assigned in the "+
				"enclosing for-statement's initialization "+
				"statement.", describeVars(vars))
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}

	if enclosingForStmt.Post != nil {
		vars := r.varsInSelectionWithReachingDefsFrom(
			enclosingForStmt.Post, defUse)
		if len(vars) > 0 {
			r.Log.Errorf("This expression cannot be extracted "+
				"because it uses %s assigned in the "+
				"for-statement's post-iteration statement.",
				describeVars(vars))
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}

	errorVars := r.findDefsInBodyReachingSelection(enclosingForStmt, defUse)
	if len(errorVars) > 0 {
		r.Log.Errorf("This expression cannot be extracted because "+
			"it uses %s assigned in the body of the for-loop.",
			describeVars(errorVars))
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}

	return true
}

// findDefsInBodyReachingSelection returns a list of variables that are
// assigned in the body of the given for-loop, where those assignments reach
// uses in the selected expression,
//
// Precondition: The selected expression is a subexpression of the given
// for-loop's condition expression.
func (r *ExtractLocal) findDefsInBodyReachingSelection(enclosingForStmt *ast.ForStmt, defUse map[ast.Stmt]map[ast.Stmt]struct{}) map[*types.Var]struct{} {
	result := map[*types.Var]struct{}{}
	ast.Inspect(enclosingForStmt.Body, func(n ast.Node) bool {
		if stmt, ok := n.(ast.Stmt); ok {
			vars := r.varsInSelectionWithReachingDefsFrom(stmt, defUse)
			for variable := range vars {
				result[variable] = struct{}{}
			}
		}
		return true
	})
	return result
}

// checkExprIsNotRangeStmtLhs determines if the selected node is either the key
// or value expression for a range statement, logging an error and returning
// false if it is.
//
// Note that it is acceptable to extract a subexpression of an LHS expression
// (e.g., the subscript expression in a[i+2]=...), but not the entire expression.
func (r *ExtractLocal) checkExprIsNotRangeStmtLhs() bool {
	for _, node := range r.PathEnclosingSelection {
		if asgt, ok := node.(*ast.RangeStmt); ok {
			if asgt.Key == r.SelectedNode || asgt.Value == r.SelectedNode {
				r.Log.Error("The selected expression cannot be extracted since it is the key or value expression for a range statement.")
				r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
				return false
			}
		}
	}
	return true
}

// checkExprIsNotInCaseClauseOfTypeSwitchStmt checks if the selected expression
// appears in a case clause for a type switch statement.  If it is, an error is
// logged, and false is returned.
//
// Precondition: r.enclosingStmtIndex() >= 0
func (r *ExtractLocal) checkExprIsNotInCaseClauseOfTypeSwitchStmt() bool {
	if _, ok := r.enclosingStmt().(*ast.CaseClause); ok {
		// grandparent will be switch or type switch statement
		grandparent := r.PathEnclosingSelection[r.enclosingStmtIndex()+2].(ast.Stmt)
		if _, ok := grandparent.(*ast.TypeSwitchStmt); ok {
			r.Log.Error("The selected expression cannot be extracted since it is in a case clause for a type switch statement.")
			r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
			return false
		}
	}
	return true
}

// checkForNameConflict determines if the new variable name will conflict with
// or shadow an existing name, logging an error and returning false if it will.
func (r *ExtractLocal) checkForNameConflict() bool {
	scope := r.scopeEnclosingSelection()
	if scope == nil {
		r.Log.Error("A scope could not be found for the selected expression.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return false
	}

	// TO DISPLAY THE SCOPE:
	// var buf bytes.Buffer
	// scope.WriteTo(&buf, 0, true)
	// fmt.Println(buf.String())

	existingObj := scope.Lookup(r.varName)
	if existingObj != nil {
		r.Log.Errorf("If a variable named %s is introduced, it will "+
			"conflict with an existing declaration.", r.varName)
		r.Log.AssociatePos(existingObj.Pos(), existingObj.Pos())
		return false
	}

	_, existingObj = scope.LookupParent(r.varName, r.SelectedNode.Pos())
	if existingObj != nil {
		r.Log.Errorf("If a variable named %s is introduced, it will "+
			"shadow an existing declaration.", r.varName)
		r.Log.AssociatePos(existingObj.Pos(), existingObj.Pos())
		return false
	}

	return true
}

// scopeEnclosingSelection returns the smallest scope in which the selected
// node exists.
func (r *ExtractLocal) scopeEnclosingSelection() *types.Scope {
	for _, node := range r.PathEnclosingSelection {
		if scope, found := r.SelectedNodePkg.Info.Scopes[node]; found {
			return scope.Innermost(r.SelectedNode.Pos())
		}
	}
	return nil
}

// findStmtToInsertBefore determines what statement the extracted variable
// assignment should be inserted before.
//
// Often, this is just the statement enclosing the selected node.  However,
// when the enclosing statement is a case clause of a switch statment, or when
// it is an if-statement that serves as the else-if of another if-statement,
// the assignment must be inserted earlier.
//
// Precondition: r.enclosingStmtIndex() >= 0
func (r *ExtractLocal) findStmtToInsertBefore() ast.Stmt {
	switch r.enclosingStmt().(type) {
	case *ast.CaseClause:
		// grandparent will be switch or type switch statement
		grandparent := r.PathEnclosingSelection[r.enclosingStmtIndex()+2].(ast.Stmt)
		return grandparent

	case *ast.IfStmt:
		idx := r.findBeginningOfElseIfChain(r.enclosingStmtIndex())
		return r.PathEnclosingSelection[idx].(ast.Stmt)

	default:
		if _, isInit := r.isIfStmtInit(); isInit {
			idx := r.findBeginningOfElseIfChain(
				r.enclosingStmtIndex() + 1)
			return r.PathEnclosingSelection[idx].(ast.Stmt)
		}

		if enclosingFor, isInit := r.isForStmtInit(); isInit {
			return enclosingFor
		}
		if enclosingFor, isPost := r.isForStmtPost(); isPost {
			return enclosingFor
		}

		return r.enclosingStmt().(ast.Stmt)
	}
}

// findBeginningOfElseIfChain searches r.PathEnclosingSelection starting at the
// given index, which should contain an *ast.IfStmt, and skips consecutive
// *ast.IfStmt entries to find the outermost enclosing *ast.IfStmt for which
// all the previous entries were else-if statements.
//
// When an if-statement appears as an "else if", possibly deeply nested, this
// finds the outermost if-statement.  The assignment to the extracted variable
// should be placed before the outermost if-statement.
//
// As the following example shows, this traces else-if chains upward.
// This is not the same as finding the outermost enclosing if-statement.
//
//     if (v) {                 // The beginning of the else-if chain is NOT v;
//         if (w) {             // it is w...
//         } else if (x) {
//             if (y) {
//             } else if (z) {  // ...if we start from z
//             }
//         }
//     }
func (r *ExtractLocal) findBeginningOfElseIfChain(index int) int {
	ifStmt := r.PathEnclosingSelection[index].(*ast.IfStmt)
	if index+1 < len(r.PathEnclosingSelection) {
		if enclosingIfStmt, ok := r.PathEnclosingSelection[index+1].(*ast.IfStmt); ok && enclosingIfStmt.Else == ifStmt {
			return r.findBeginningOfElseIfChain(index + 1)
		}
	}
	return index
}

// addEdits adds source code edits for this refactoring
func (r *ExtractLocal) addEdits(insertBefore ast.Stmt) {
	selectedExprOffset := r.getOffset(r.SelectedNode)
	selectedExprEnd := r.getEndOffset(r.SelectedNode)
	selectedExprLen := selectedExprEnd - selectedExprOffset

	// First, replace the original expression.
	r.Edits[r.Filename].Add(&text.Extent{selectedExprOffset, selectedExprLen}, r.varName)

	// Then, add the assignment statement afterward.
	// If this inserts at the same position as the replacement, this
	// guarantees that it will be inserted before it, which is what we want
	expression := string(r.FileContents[selectedExprOffset:selectedExprEnd])
	assignment := r.varName + " := " + expression + "\n"
	r.Edits[r.Filename].Add(&text.Extent{r.getOffset(insertBefore), 0}, assignment)
}

// getOffset returns the token.Pos for the first character of the given node
func (r *ExtractLocal) getOffset(node ast.Node) int {
	return r.Program.Fset.Position(node.Pos()).Offset
}

// getEndOffset returns the token.Pos one byte beyond the end of the given node
func (r *ExtractLocal) getEndOffset(node ast.Node) int {
	return r.Program.Fset.Position(node.End()).Offset
}

const extractLocalDoc = `
  <h4>Purpose</h4>
  <p>The Extract Local Variable takes an expression, assigns it to a new
  local variable, then replaces the original expression with a use of that
  variable.</p>

  <h4>Usage</h4>
  <ol class="enum">
    <li>Select an expression in an existing statement.</li>
    <li>Activate the Extract Local Variable refactoring.</li>
    <li>Enter a name for the new variable that will be created.</li>
  </ol>

  <p>An error or warning will be reported if the selected expression cannot be
  extracted into a variable assignment.  For example, this could occur if the
  extracted expression is in a loop condition but its value may change on each
  iteration of the loop, or if the extracted variable's name is the same as the
  name of an existing variable.</p>

  <h4>Example</h4>
  <p>The example below demonstrates the effect of extracting the highlighted
  expression into a new local variable <tt>sum</tt>.</p>
  <table cellspacing="5" cellpadding="15" style="border: 0;">
    <tr>
      <th>Before</th><th>&nbsp;</th><th>After</th>
    </tr>
    <tr>
      <td class="dotted">
        <pre>package main
import "fmt"

func main() {
    y := 2
    z := 3
    if x := 5; x == <span class="highlight">y + z</span> {
        fmt.Println(x)
    }
}</pre>
      </td>
      <td>&nbsp;&nbsp;&nbsp;&nbsp;&rArr;&nbsp&nbsp;&nbsp;&nbsp;</td>
      <td class="dotted">
        <pre>package main
import "fmt"

func main() {
    y := 2
    z := 3
    <span class="highlight">sum := y + z</span>
    if x := 5; x == <span class="highlight">sum</span> {
        fmt.Println(x)
    }
}</pre>
      </td>
    </tr>
  </table>
`
