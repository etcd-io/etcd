package errorlint

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/constant"
	"go/printer"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

type ByPosition []analysis.Diagnostic

func (l ByPosition) Len() int      { return len(l) }
func (l ByPosition) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
func (l ByPosition) Less(i, j int) bool {
	return l[i].Pos < l[j].Pos
}

func LintFmtErrorfCalls(fset *token.FileSet, info types.Info, multipleWraps bool) []analysis.Diagnostic {
	var lints []analysis.Diagnostic

	for expr, t := range info.Types {
		// Search for error expressions that are the result of fmt.Errorf
		// invocations.
		if t.Type.String() != "error" {
			continue
		}
		call, ok := isFmtErrorfCallExpr(info, expr)
		if !ok {
			continue
		}

		// Find all % fields in the format string.
		formatVerbs, ok := printfFormatStringVerbs(info, call)
		if !ok {
			continue
		}

		// For any arguments that are errors, check whether the wrapping verb is used. %w may occur
		// for multiple errors in one Errorf invocation, unless multipleWraps is true. We raise an
		// issue if at least one error does not have a corresponding wrapping verb.
		args := call.Args[1:]
		if !multipleWraps {
			wrapCount := 0
			for i := 0; i < len(args) && i < len(formatVerbs); i++ {
				arg := args[i]
				if !implementsError(info.Types[arg].Type) {
					continue
				}
				verb := formatVerbs[i]

				if verb.format == "w" {
					wrapCount++
					if wrapCount > 1 {
						lints = append(lints, analysis.Diagnostic{
							Message: "only one %w verb is permitted per format string",
							Pos:     arg.Pos(),
						})
						break
					}
				}

				if wrapCount == 0 {
					lints = append(lints, analysis.Diagnostic{
						Message: "non-wrapping format verb for fmt.Errorf. Use `%w` to format errors",
						Pos:     args[i].Pos(),
					})
					break
				}
			}

		} else {
			var lint *analysis.Diagnostic
			argIndex := 0
			for _, verb := range formatVerbs {
				if verb.index != -1 {
					argIndex = verb.index
				} else {
					argIndex++
				}

				if verb.format == "w" || verb.format == "T" {
					continue
				}
				if argIndex-1 >= len(args) {
					continue
				}
				arg := args[argIndex-1]
				if !implementsError(info.Types[arg].Type) {
					continue
				}

				strStart := call.Args[0].Pos()
				if lint == nil {
					lint = &analysis.Diagnostic{
						Message: "non-wrapping format verb for fmt.Errorf. Use `%w` to format errors",
						Pos:     arg.Pos(),
					}
				}
				fixMessage := "Use `%w` to format errors"
				if len(lint.SuggestedFixes) > 0 {
					fixMessage += fmt.Sprintf(" (%d)", len(lint.SuggestedFixes)+1)
				}
				lint.SuggestedFixes = append(lint.SuggestedFixes, analysis.SuggestedFix{
					Message: fixMessage,
					TextEdits: []analysis.TextEdit{{
						Pos:     strStart + token.Pos(verb.formatOffset) + 1,
						End:     strStart + token.Pos(verb.formatOffset) + 2,
						NewText: []byte("w"),
					}},
				})
			}
			if lint != nil {
				lints = append(lints, *lint)
			}
		}
	}
	return lints
}

// printfFormatStringVerbs returns a normalized list of all the verbs that are used per argument to
// the printf function. The index of each returned element corresponds to the index of the
// respective argument.
func printfFormatStringVerbs(info types.Info, call *ast.CallExpr) ([]verb, bool) {
	if len(call.Args) <= 1 {
		return nil, false
	}
	strLit, ok := call.Args[0].(*ast.BasicLit)
	if !ok {
		// Ignore format strings that are not literals.
		return nil, false
	}
	formatString := constant.StringVal(info.Types[strLit].Value)

	pp := printfParser{str: formatString}
	verbs, err := pp.ParseAllVerbs()
	if err != nil {
		return nil, false
	}

	return verbs, true
}

func isFmtErrorfCallExpr(info types.Info, expr ast.Expr) (*ast.CallExpr, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	fn, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		// TODO: Support fmt.Errorf variable aliases?
		return nil, false
	}
	obj := info.Uses[fn.Sel]

	pkg := obj.Pkg()
	if pkg != nil && pkg.Name() == "fmt" && obj.Name() == "Errorf" {
		return call, true
	}
	return nil, false
}

func LintErrorComparisons(info *TypesInfoExt) []analysis.Diagnostic {
	var lints []analysis.Diagnostic

	// Check for error comparisons.
	for expr := range info.TypesInfo.Types {
		// Find == and != operations.
		binExpr, ok := expr.(*ast.BinaryExpr)
		if !ok {
			continue
		}
		if binExpr.Op != token.EQL && binExpr.Op != token.NEQ {
			continue
		}
		// Comparing errors with nil is okay.
		if isNil(binExpr.X) || isNil(binExpr.Y) {
			continue
		}
		// Find comparisons of which one side is a of type error.
		if !isErrorType(info.TypesInfo, binExpr.X) && !isErrorType(info.TypesInfo, binExpr.Y) {
			continue
		}
		// Some errors that are returned from some functions are exempt.
		if isAllowedErrorComparison(info, binExpr.X, binExpr.Y) {
			continue
		}
		// Comparisons that happen in `func (type) Is(error) bool` are okay.
		if isNodeInErrorIsFunc(info, binExpr) {
			continue
		}

		diagnostic := analysis.Diagnostic{
			Message: fmt.Sprintf("comparing with %s will fail on wrapped errors. Use errors.Is to check for a specific error", binExpr.Op),
			Pos:     binExpr.Pos(),
		}

		// Add suggested fix.
		var errVar, targetErr ast.Expr
		// Identify which side is the error variable and which is the sentinel error.
		if isErrorType(info.TypesInfo, binExpr.Y) && !isErrorType(info.TypesInfo, binExpr.X) {
			// Y is error, X is not
			errVar = binExpr.Y
			targetErr = binExpr.X
		} else {
			// X is error (or both are errors)
			errVar = binExpr.X
			targetErr = binExpr.Y
		}

		negated := binExpr.Op == token.NEQ

		// Build the suggested fix - preserve the original order of parameters.
		replacement := fmt.Sprintf("errors.Is(%s, %s)", exprToString(errVar), exprToString(targetErr))
		if negated {
			replacement = "!" + replacement
		}

		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Use errors.Is() to compare errors",
			TextEdits: []analysis.TextEdit{{
				Pos:     binExpr.Pos(),
				End:     binExpr.End(),
				NewText: []byte(replacement),
			}},
		}}

		lints = append(lints, diagnostic)
	}

	// Check for error comparisons in switch statements.
	for scope := range info.TypesInfo.Scopes {
		// Find value switch blocks.
		switchStmt, ok := scope.(*ast.SwitchStmt)
		if !ok {
			continue
		}
		// Check whether the switch operates on an error type.
		if !isErrorType(info.TypesInfo, switchStmt.Tag) {
			continue
		}

		var problematicCaseClause *ast.CaseClause
	outer:
		for _, stmt := range switchStmt.Body.List {
			caseClause := stmt.(*ast.CaseClause)
			for _, caseExpr := range caseClause.List {
				if isNil(caseExpr) {
					continue
				}
				// Some errors that are returned from some functions are exempt.
				if !isAllowedErrorComparison(info, switchStmt.Tag, caseExpr) {
					problematicCaseClause = caseClause
					break outer
				}
			}
		}
		if problematicCaseClause == nil {
			continue
		}

		// Comparisons that happen in `func (type) Is(error) bool` are okay.
		if isNodeInErrorIsFunc(info, switchStmt) {
			continue
		}

		if !switchComparesNonNil(switchStmt) {
			continue
		}

		diagnostic := analysis.Diagnostic{
			Message: "switch on an error will fail on wrapped errors. Use errors.Is to check for specific errors",
			Pos:     problematicCaseClause.Pos(),
		}

		// Create a simpler version of the fix for switch statements
		// We'll transform: switch err { case ErrX: ... }
		// To:             switch { case errors.Is(err, ErrX): ... }

		// Create a new switch statement with an empty tag
		newSwitchStmt := &ast.SwitchStmt{
			Init: switchStmt.Init,
			Tag:  nil, // Empty tag for the switch.
			Body: &ast.BlockStmt{
				List: make([]ast.Stmt, len(switchStmt.Body.List)),
			},
		}

		// Convert each case to use errors.Is.
		switchTagExpr := switchStmt.Tag // The error variable being checked.
		for i, stmt := range switchStmt.Body.List {
			origCaseClause := stmt.(*ast.CaseClause)

			// Create a new case clause.
			newCaseClause := &ast.CaseClause{
				Body: origCaseClause.Body,
			}

			// If this is a default case (no expressions), keep it as-is.
			if len(origCaseClause.List) == 0 {
				newCaseClause.List = nil // Default case.
				newSwitchStmt.Body.List[i] = newCaseClause
				continue
			}

			newCaseClause.List = make([]ast.Expr, 0, len(origCaseClause.List))

			// Convert each case expression.
			for _, caseExpr := range origCaseClause.List {
				if isNil(caseExpr) {
					// Keep nil checks as is: case err == nil:
					newCaseClause.List = append(newCaseClause.List,
						&ast.BinaryExpr{
							X:  switchTagExpr,
							Op: token.EQL,
							Y:  caseExpr,
						})
					continue
				}
				// Replace err == ErrX with errors.Is(err, ErrX).
				newCaseClause.List = append(newCaseClause.List,
					&ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   ast.NewIdent("errors"),
							Sel: ast.NewIdent("Is"),
						},
						Args: []ast.Expr{switchTagExpr, caseExpr},
					})
			}

			newSwitchStmt.Body.List[i] = newCaseClause
		}

		// Print the modified AST to get the fix text.
		var buf bytes.Buffer
		printer.Fprint(&buf, token.NewFileSet(), newSwitchStmt)
		fixText := buf.String()

		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Convert to errors.Is() for error comparisons",
			TextEdits: []analysis.TextEdit{{
				Pos:     switchStmt.Pos(),
				End:     switchStmt.End(),
				NewText: []byte(fixText),
			}},
		}}

		lints = append(lints, diagnostic)
	}

	return lints
}

// exprToString converts an expression to its string representation.
func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.UnaryExpr:
		return e.Op.String() + exprToString(e.X)
	case *ast.BinaryExpr:
		return exprToString(e.X) + " " + e.Op.String() + " " + exprToString(e.Y)
	case *ast.CallExpr:
		var args []string
		for _, arg := range e.Args {
			args = append(args, exprToString(arg))
		}
		return exprToString(e.Fun) + "(" + strings.Join(args, ", ") + ")"
	case *ast.ParenExpr:
		return "(" + exprToString(e.X) + ")"
	case *ast.IndexExpr:
		return exprToString(e.X) + "[" + exprToString(e.Index) + "]"
	case *ast.BasicLit:
		return e.Value
	case *ast.TypeAssertExpr:
		return exprToString(e.X) + ".(" + exprToString(e.Type) + ")"
	default:
		// If we can't handle the expression type, return a placeholder.
		return "/* complex expression */"
	}
}

func isNil(ex ast.Expr) bool {
	ident, ok := ex.(*ast.Ident)
	return ok && ident.Name == "nil"
}

func isErrorType(info *types.Info, ex ast.Expr) bool {
	t := info.Types[ex].Type
	return t != nil && t.String() == "error"
}

func isNodeInErrorIsFunc(info *TypesInfoExt, node ast.Node) bool {
	funcDecl := info.ContainingFuncDecl(node)
	if funcDecl == nil {
		return false
	}
	// Check if the function name is Is.
	if funcDecl.Name.Name != "Is" {
		return false
	}
	// Check if the function has a receiver.
	if funcDecl.Recv == nil {
		return false
	}
	// There should be 1 argument of type error.
	if params := funcDecl.Type.Params.List; len(params) != 1 || info.TypesInfo.Types[params[0].Type].Type.String() != "error" {
		return false
	}
	// The return type should be bool.
	if params := funcDecl.Type.Results.List; len(params) != 1 || info.TypesInfo.Types[params[0].Type].Type.String() != "bool" {
		return false
	}
	return true
}

// switchComparesNonNil returns true if one of its clauses compares by value.
func switchComparesNonNil(switchStmt *ast.SwitchStmt) bool {
	for _, caseBlock := range switchStmt.Body.List {
		caseClause, ok := caseBlock.(*ast.CaseClause)
		if !ok {
			continue
		}
		for _, clause := range caseClause.List {
			switch clause := clause.(type) {
			case nil:
				// default label is safe.
				continue
			case *ast.Ident:
				// `case nil` is safe.
				if clause.Name == "nil" {
					continue
				}
			}
			// anything else (including an Ident other than nil) isn't safe.
			return true
		}
	}
	return false
}

func LintErrorTypeAssertions(fset *token.FileSet, info *TypesInfoExt) []analysis.Diagnostic {
	var lints []analysis.Diagnostic

	for expr := range info.TypesInfo.Types {
		// Find type assertions.
		typeAssert, ok := expr.(*ast.TypeAssertExpr)
		if !ok {
			continue
		}

		// Find type assertions that operate on values of type error.
		if !isErrorTypeAssertion(*info.TypesInfo, typeAssert) {
			continue
		}

		if isNodeInErrorIsFunc(info, typeAssert) {
			continue
		}

		// If the asserted type is not an error, allow the expression.
		if !implementsError(info.TypesInfo.Types[typeAssert.Type].Type) {
			continue
		}

		diagnostic := analysis.Diagnostic{
			Message: "type assertion on error will fail on wrapped errors. Use errors.As to check for specific errors",
			Pos:     typeAssert.Pos(),
		}

		// Create suggested fix for type assertion
		targetType := exprToString(typeAssert.Type)
		errExpr := exprToString(typeAssert.X)

		// Check if the type is a pointer type
		baseType, isPointerType := strings.CutPrefix(targetType, "*")

		parent := info.NodeParent[typeAssert]

		// For assignment statements like: targetErr, ok := err.(*SomeError)
		if assign, ok := parent.(*ast.AssignStmt); ok && len(assign.Lhs) == 2 {
			if id, ok := assign.Lhs[0].(*ast.Ident); ok {
				// Generate a suitable variable name, handling underscore case
				// Example: _, ok := err.(*MyError) -> myError := &MyError{}; ok := errors.As(err, &myError)
				varName := generateErrorVarName(id.Name, baseType)

				// If this is part of an if statement initialization
				ifParent, isIfInit := info.NodeParent[assign].(*ast.IfStmt)
				if isIfInit && ifParent.Init == assign {
					// Handle special case for if statements
					// Replace: if targetErr, ok := err.(*SomeError); ok {
					// With:    targetErr := &SomeError{}
					//          if errors.As(err, &targetErr) {
					var varDecl string
					if isPointerType {
						varDecl = fmt.Sprintf("%s := &%s{}", varName, baseType)
					} else {
						varDecl = fmt.Sprintf("var %s %s", varName, baseType)
					}
					condition := fmt.Sprintf("if errors.As(%s, &%s)", errExpr, varName)

					replacement := fmt.Sprintf("%s\n%s",
						varDecl, condition)

					diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
						Message: "Use errors.As() for type assertions on errors",
						TextEdits: []analysis.TextEdit{{
							// Replace both the if statement's initialization and condition
							Pos:     ifParent.Pos(),
							End:     ifParent.Body.Pos(),
							NewText: []byte(replacement),
						}},
					}}
					lints = append(lints, diagnostic)
					continue
				}

				// Regular assignment outside of if statement.
				// Replace: targetErr, ok := err.(*SomeError) or err.(SomeError)
				// With:    targetErr := &SomeError{} or var targetErr SomeError
				//          ok := errors.As(err, &targetErr)
				var varDecl string
				if isPointerType {
					varDecl = fmt.Sprintf("%s := &%s{}", varName, baseType)
				} else {
					varDecl = fmt.Sprintf("var %s %s", varName, baseType)
				}

				// Preserve the original name of the "ok" variable
				// Example: myErr, wasFound := err.(*MyError)
				// Should use "wasFound" in the transformed code, not just "ok"
				okName := "ok" // Default
				if len(assign.Lhs) > 1 {
					if okIdent, okOk := assign.Lhs[1].(*ast.Ident); okOk && okIdent.Name != "_" {
						okName = okIdent.Name
					}
				}

				// Align with golden file format
				replacement := fmt.Sprintf("%s\n%s := errors.As(%s, &%s)",
					varDecl, okName, errExpr, varName)

				diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
					Message: "Use errors.As() for type assertions on errors",
					TextEdits: []analysis.TextEdit{{
						Pos:     assign.Pos(),
						End:     assign.End(),
						NewText: []byte(replacement),
					}},
				}}
				lints = append(lints, diagnostic)
				continue
			}
		}

		if _, ok := parent.(*ast.IfStmt); ok {
			// For if statements without initialization but with direct type assertion in condition
			varName := generateErrorVarName("target", baseType)
			var varDecl string
			if isPointerType {
				varDecl = fmt.Sprintf("%s := &%s{}", varName, baseType)
			} else {
				varDecl = fmt.Sprintf("var %s %s", varName, baseType)
			}
			replacement := fmt.Sprintf("%s\nif errors.As(%s, &%s)",
				varDecl, errExpr, varName)

			diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
				Message: "Use errors.As() for type assertions on errors",
				TextEdits: []analysis.TextEdit{{
					Pos:     typeAssert.Pos(),
					End:     typeAssert.End(),
					NewText: []byte(replacement),
				}},
			}}
			lints = append(lints, diagnostic)
			continue
		}

		// Handle standalone type assertions without assignment
		// Example: _ = err.(*MyError)
		// Transforms to: _ = func() *MyError { var target *MyError; _ = errors.As(err, &target); return target }()
		varName := generateErrorVarName("target", baseType)
		var targetDecl string
		if isPointerType {
			targetDecl = fmt.Sprintf("%s := &%s{}", varName, baseType)
		} else {
			targetDecl = fmt.Sprintf("var %s %s", varName, baseType)
		}

		replacement := fmt.Sprintf("func() %s {\n\t%s\n\t_ = errors.As(%s, &%s)\n\treturn %s\n}()",
			targetType, targetDecl, errExpr, varName, varName)

		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Use errors.As() for type assertions on errors",
			TextEdits: []analysis.TextEdit{{
				Pos:     typeAssert.Pos(),
				End:     typeAssert.End(),
				NewText: []byte(replacement),
			}},
		}}
		lints = append(lints, diagnostic)
	}

	for scope := range info.TypesInfo.Scopes {
		// Find type switches.
		typeSwitch, ok := scope.(*ast.TypeSwitchStmt)
		if !ok {
			continue
		}

		// Find the type assertion in the type switch.
		var typeAssert *ast.TypeAssertExpr
		switch t := typeSwitch.Assign.(type) {
		case *ast.ExprStmt:
			typeAssert = t.X.(*ast.TypeAssertExpr)
		case *ast.AssignStmt:
			typeAssert = t.Rhs[0].(*ast.TypeAssertExpr)
		}

		// Check whether the type switch is on a value of type error.
		if !isErrorTypeAssertion(*info.TypesInfo, typeAssert) {
			continue
		}

		if isNodeInErrorIsFunc(info, typeSwitch) {
			continue
		}

		diagnostic := analysis.Diagnostic{
			Message: "type switch on error will fail on wrapped errors. Use errors.As to check for specific errors",
			Pos:     typeAssert.Pos(),
		}

		// Transform type switch into a switch statement with errors.As in each case
		// e.g., switch err.(type) { case *MyError: ... } becomes:
		// var myError *MyError; switch { case errors.As(err, &myError): ... }

		// Get the error variable being type-switched on
		errExpr := typeAssert.X

		// a flag to know if we can fix the issue with a [analysis.SuggestedFix]
		canFix := true

		// Determine if this is a type switch with assignment (switch e := err.(type))
		var assignIdent *ast.Ident
		var useShadowVar bool
		if assignStmt, ok := typeSwitch.Assign.(*ast.AssignStmt); ok {
			// This is a type switch with assignment like: switch e := err.(type)
			if len(assignStmt.Lhs) == 1 {
				if id, ok := assignStmt.Lhs[0].(*ast.Ident); ok {
					if exprToString(errExpr) == id.Name {
						// the switch with assignment is like switch err := err.(type)
						// we cannot reuse err, otherwise it would lead to errors(err, &err)
						canFix = false

						// TODO - suggest a fix with a new variable name instead?
						// the issue is with the fact each branch should have a new variable name
					} else {
						// the variable names are different, we can reuse the assigned variable
						assignIdent = id
						useShadowVar = true
					}
				}
			}
		}

		// Create variable declarations for each type
		varDecls := []ast.Stmt{}

		// Create a map of type expressions to variable names
		typeToVar := make(map[ast.Expr]string)

		// First collect all unique types from cases
		caseTypes := []ast.Expr{}
		for _, stmt := range typeSwitch.Body.List {
			caseClause := stmt.(*ast.CaseClause)
			for _, typeExpr := range caseClause.List {
				// Skip default case (empty list) and nil comparisons.
				if typeExpr != nil && !isNil(typeExpr) {
					caseTypes = append(caseTypes, typeExpr)
				}
			}
		}

		// Create variable declarations for each type.
		for i, typeExpr := range caseTypes {
			// Create variable declarations for each type.
			// generate a default and unique name
			varName := fmt.Sprintf("errCase%d", i)

			// then try to find a better one.
			if useShadowVar || (assignIdent != nil && i == 0) {
				// If we have an assignment identifier, use it for all variables in a switch with assignment.
				// Otherwise, if we have an assignment but not shadowing, use it for the first variable.
				varName = assignIdent.Name
			}

			// Ensure we don't create duplicate variables with the same name.
			var duplicate bool
			for j := 0; j < i; j++ {
				if typeToVar[caseTypes[j]] == varName {
					duplicate = true
					break
				}
			}

			if duplicate {
				// Use a different name to avoid duplicate variable declarations.
				varName = fmt.Sprintf("%s%d", varName, i)
			}

			typeToVar[typeExpr] = varName

			// Create a variable declaration.
			varDecl := &ast.DeclStmt{
				Decl: &ast.GenDecl{
					Tok: token.VAR,
					Specs: []ast.Spec{
						&ast.ValueSpec{
							Names: []*ast.Ident{ast.NewIdent(varName)},
							Type:  typeExpr,
						},
					},
				},
			}

			varDecls = append(varDecls, varDecl)
		}

		// Create a new switch statement with empty tag.
		newSwitchStmt := &ast.SwitchStmt{
			Body: &ast.BlockStmt{
				List: make([]ast.Stmt, len(typeSwitch.Body.List)),
			},
		}

		// Create a block statement to hold both variable declarations and the switch.
		blockStmt := &ast.BlockStmt{
			List: append(varDecls, newSwitchStmt),
		}

		// Process each case.
		for i, stmt := range typeSwitch.Body.List {
			caseClause := stmt.(*ast.CaseClause)

			// Create a new case clause.
			newCaseClause := &ast.CaseClause{
				Body: caseClause.Body,
			}

			// If this is a default case, keep it as-is.
			if len(caseClause.List) == 0 {
				// This is the default case.
				newCaseClause.List = nil
				newSwitchStmt.Body.List[i] = newCaseClause
				continue
			}

			// For other cases, create errors.As calls for each type.
			newCaseClause.List = make([]ast.Expr, len(caseClause.List))

			for j, typeExpr := range caseClause.List {
				// Nil cases should become err == nil.
				if isNil(typeExpr) {
					newCaseClause.List[j] = &ast.BinaryExpr{
						X:  errExpr,
						Op: token.EQL,
						Y:  typeExpr,
					}
					continue
				}

				// Get the previously declared variable for this type.
				varName := typeToVar[typeExpr]

				// Create errors.As(err, &varName) call.
				newCaseClause.List[j] = &ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   ast.NewIdent("errors"),
						Sel: ast.NewIdent("As"),
					},
					Args: []ast.Expr{
						errExpr,
						&ast.UnaryExpr{
							Op: token.AND,
							X:  ast.NewIdent(varName),
						},
					},
				}
			}

			// If this is a switch with assignment, we need to update the variable
			// names used in the body of each case to match our renamed variables.
			if assignIdent != nil && len(caseClause.List) > 0 {
				typeExpr := caseClause.List[0]
				oldVarName := assignIdent.Name
				newVarName := typeToVar[typeExpr]

				if oldVarName != newVarName {
					// Create a visitor to replace all mentions of the original variable
					// with our renamed variable in this case's body.
					visitor := func(n ast.Node) bool {
						if ident, ok := n.(*ast.Ident); ok && ident.Name == oldVarName {
							ident.Name = newVarName
						}
						return true
					}

					// Apply the visitor to the case body
					for _, bodyStmt := range newCaseClause.Body {
						ast.Inspect(bodyStmt, visitor)
					}
				}
			}

			// Add this case to the switch.
			newSwitchStmt.Body.List[i] = newCaseClause
		}

		if canFix {
			// Print the resulting block to get the fix text.
			var buf bytes.Buffer
			printer.Fprint(&buf, token.NewFileSet(), blockStmt)
			fixText := buf.String()

			diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
				Message: "Convert type switch to use errors.As",
				TextEdits: []analysis.TextEdit{{
					Pos:     typeSwitch.Pos(),
					End:     typeSwitch.End(),
					NewText: []byte(fixText),
				}},
			}}
		}
		lints = append(lints, diagnostic)
	}

	return lints
}

func isErrorTypeAssertion(info types.Info, typeAssert *ast.TypeAssertExpr) bool {
	t := info.Types[typeAssert.X]
	return t.Type.String() == "error"
}

func implementsError(t types.Type) bool {
	mset := types.NewMethodSet(t)

	for i := 0; i < mset.Len(); i++ {
		if mset.At(i).Kind() != types.MethodVal {
			continue
		}

		obj := mset.At(i).Obj()
		if obj.Name() == "Error" && obj.Type().String() == "func() string" {
			return true
		}
	}

	return false
}

// generateErrorVarName creates an appropriate variable name for error type assertions.
// If originalName is "_" or a generic placeholder, it generates a more meaningful name
// based on the error type, following Go naming conventions:
//
// Examples:
//   - originalName="_", typeName="MyError" → "myError" (camelCase conversion)
//   - originalName="_", typeName="pkg.CustomError" → "customError" (package prefix removed)
//   - originalName="existingName" → "existingName" (original name preserved)
//   - originalName="_", typeName="" → "myErr" (fallback for unknown types)
//
// This helps ensure code readability when converting type assertions to errors.As calls,
// particularly when dealing with underscore identifiers that can't be referenced.
func generateErrorVarName(originalName, typeName string) string {
	// If the original name is not an underscore, use it
	if originalName != "_" {
		return originalName
	}

	// Handle underscore case by generating a name based on the type
	// Strip any package prefix like "pkg."
	if lastDot := strings.LastIndex(typeName, "."); lastDot >= 0 {
		typeName = typeName[lastDot+1:]
	}

	// Convert first letter to lowercase for camelCase
	if len(typeName) > 0 {
		firstChar := strings.ToLower(typeName[:1])
		if len(typeName) > 1 {
			return firstChar + typeName[1:]
		}
		return firstChar
	}

	// If we couldn't determine a good name, use default.
	return "anErr"
}
