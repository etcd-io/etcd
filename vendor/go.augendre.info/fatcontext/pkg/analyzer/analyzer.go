// Package analyzer contains everything related to the linter analysis.
package analyzer

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// FlagCheckStructPointers is a possible flag for the analyzer.
// Exported to make it usable in golangci-lint.
const FlagCheckStructPointers = "check-struct-pointers"

// NewAnalyzer returns a fatcontext analyzer.
func NewAnalyzer() *analysis.Analyzer {
	rnnr := &runner{}

	flags := flag.NewFlagSet("fatcontext", flag.ExitOnError)
	flags.BoolVar(&rnnr.DetectInStructPointers, FlagCheckStructPointers, false,
		"set to true to detect potential fat contexts in struct pointers")

	return &analysis.Analyzer{
		Name:     "fatcontext",
		Doc:      "detects nested contexts in loops and function literals",
		Run:      rnnr.run,
		Flags:    *flags,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}
}

var (
	errUnknown         = errors.New("unknown node type")
	errInvalidAnalysis = errors.New("invalid analysis")
)

const (
	categoryInLoop          = "nested context in loop"
	categoryInFuncLit       = "nested context in function literal"
	categoryInStructPointer = "potential nested context in struct pointer"
	categoryUnsupported     = "unsupported nested context type"
)

type runner struct {
	DetectInStructPointers bool
}

func (r *runner) run(pass *analysis.Pass) (interface{}, error) {
	inspctr, typeValid := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !typeValid {
		return nil, errInvalidAnalysis
	}

	nodeFilter := []ast.Node{
		(*ast.ForStmt)(nil),
		(*ast.RangeStmt)(nil),
		(*ast.FuncLit)(nil),
		(*ast.FuncDecl)(nil),
	}

	inspctr.Preorder(nodeFilter, func(node ast.Node) {
		body, err := getBody(node)
		if err != nil {
			return
		}

		if body == nil {
			return
		}

		assignStmt := findNestedContext(pass, node, body.List)
		if assignStmt == nil {
			return
		}

		category := getCategory(pass, node, assignStmt)

		if r.shouldIgnoreReport(category) {
			return
		}

		fixes := r.getSuggestedFixes(pass, assignStmt, category)

		pass.Report(analysis.Diagnostic{
			Pos:            assignStmt.Pos(),
			Message:        category,
			SuggestedFixes: fixes,
		})
	})

	return nil, nil //nolint:nilnil // we have no result to send to other analyzers
}

func (r *runner) shouldIgnoreReport(category string) bool {
	return category == categoryInStructPointer && !r.DetectInStructPointers
}

func (r *runner) getSuggestedFixes(
	pass *analysis.Pass,
	assignStmt *ast.AssignStmt,
	category string,
) []analysis.SuggestedFix {
	switch category {
	case categoryInStructPointer, categoryUnsupported:
		return nil
	}

	suggestedStmt := ast.AssignStmt{
		Lhs:    assignStmt.Lhs,
		TokPos: assignStmt.TokPos,
		Tok:    token.DEFINE,
		Rhs:    assignStmt.Rhs,
	}
	suggested, err := render(pass.Fset, &suggestedStmt)

	var fixes []analysis.SuggestedFix
	if err == nil {
		fixes = append(fixes, analysis.SuggestedFix{
			Message: "replace `=` with `:=`",
			TextEdits: []analysis.TextEdit{
				{
					Pos:     assignStmt.Pos(),
					End:     assignStmt.End(),
					NewText: suggested,
				},
			},
		})
	}

	return fixes
}

func getCategory(pass *analysis.Pass, node ast.Node, assignStmt *ast.AssignStmt) string {
	switch node.(type) {
	case *ast.ForStmt, *ast.RangeStmt:
		return categoryInLoop
	}

	if isPointer(pass, assignStmt.Lhs[0]) {
		return categoryInStructPointer
	}

	switch node.(type) {
	case *ast.FuncLit, *ast.FuncDecl:
		return categoryInFuncLit
	default:
		return categoryUnsupported
	}
}

func getBody(node ast.Node) (*ast.BlockStmt, error) {
	switch typedNode := node.(type) {
	case *ast.ForStmt:
		return typedNode.Body, nil
	case *ast.RangeStmt:
		return typedNode.Body, nil
	case *ast.FuncLit:
		return typedNode.Body, nil
	case *ast.FuncDecl:
		return typedNode.Body, nil
	}

	return nil, errUnknown
}

func findNestedContext(pass *analysis.Pass, node ast.Node, stmts []ast.Stmt) *ast.AssignStmt {
	// Track which context variables have been reset to a known empty context
	resetContexts := make(map[string]bool)

	for _, stmt := range stmts {
		// Recurse if necessary
		stmtList := getStmtList(stmt)
		if found := findNestedContext(pass, node, stmtList); found != nil {
			return found
		}

		// Actually check for nested context
		assignStmt, ok := stmt.(*ast.AssignStmt)
		if !ok {
			continue
		}

		t := pass.TypesInfo.TypeOf(assignStmt.Lhs[0])
		if t == nil {
			continue
		}

		if t.String() != "context.Context" {
			continue
		}

		if assignStmt.Tok == token.DEFINE {
			continue
		}

		// Get the variable name being assigned to
		varName := getVarName(pass, assignStmt)

		// If the assignment is to a known empty context, mark this variable as reset
		if isEmptyContext(assignStmt.Rhs[0]) {
			if varName != "" {
				resetContexts[varName] = true
			}

			continue
		}

		// If this variable was previously reset to a known empty context in this block,
		// it's safe to modify it
		if varName != "" && resetContexts[varName] {
			continue
		}

		if isPointer(pass, assignStmt.Lhs[0]) {
			return assignStmt
		}

		// allow assignment to non-pointer children of values defined within the loop
		if isWithinLoop(assignStmt.Lhs[0], node, pass) {
			continue
		}

		return assignStmt
	}

	return nil
}

func getVarName(pass *analysis.Pass, assignStmt *ast.AssignStmt) string {
	varName := ""

	if ident, ok := assignStmt.Lhs[0].(*ast.Ident); ok {
		varName = ident.Name
	} else if sel, ok := assignStmt.Lhs[0].(*ast.SelectorExpr); ok {
		// For struct fields like tc.ctx
		if rendered, err := render(pass.Fset, sel); err == nil {
			varName = string(rendered)
		}
	}

	return varName
}

func getStmtList(stmt ast.Stmt) []ast.Stmt {
	switch typedStmt := stmt.(type) {
	case *ast.BlockStmt:
		return typedStmt.List
	case *ast.IfStmt:
		return typedStmt.Body.List
	case *ast.SwitchStmt:
		return typedStmt.Body.List
	case *ast.CaseClause:
		return typedStmt.Body
	case *ast.SelectStmt:
		return typedStmt.Body.List
	case *ast.CommClause:
		return typedStmt.Body
	}

	return nil
}

// render returns the pretty-print of the given node.
func render(fset *token.FileSet, x interface{}) ([]byte, error) {
	var buf bytes.Buffer

	err := printer.Fprint(&buf, fset, x)
	if err != nil {
		return nil, fmt.Errorf("printing node: %w", err)
	}

	return buf.Bytes(), nil
}

func isEmptyContext(exp ast.Expr) bool {
	call, typeValid := exp.(*ast.CallExpr)
	if !typeValid {
		return false
	}

	selector, typeValid := call.Fun.(*ast.SelectorExpr)
	if !typeValid {
		return false
	}

	ident, typeValid := selector.X.(*ast.Ident)
	if !typeValid {
		return false
	}

	isContextPackage := ident.Name == "context"
	isSafeFunc := selector.Sel.Name == "Background" || selector.Sel.Name == "TODO"

	// context.Background and context.TODO are safe
	if isContextPackage && isSafeFunc {
		return true
	}

	// Looking for a call to testing.T.Context or testing.B.Context or testing.TB.Context.
	// Checking if the called function is Context, to avoid unnecessary work.
	if selector.Sel.Name != "Context" {
		return false
	}

	if ident.Obj == nil || ident.Obj.Decl == nil {
		return false
	}

	decl, typeValid := ident.Obj.Decl.(*ast.Field)
	if !typeValid {
		return false
	}

	// Unpack the StarExpr if necessary (as in *testing.T vs. testing.TB)
	//             This makes it a StarExpr ---^
	var declType any = decl.Type

	if star, typeValid := decl.Type.(*ast.StarExpr); typeValid {
		declType = star.X
	}

	declSelector, typeValid := declType.(*ast.SelectorExpr)
	if !typeValid {
		return false
	}

	declIdent, typeValid := declSelector.X.(*ast.Ident)
	if !typeValid {
		return false
	}

	isTestingPackage := declIdent.Name == "testing"
	isValidType := declSelector.Sel.Name == "T" ||
		declSelector.Sel.Name == "B" ||
		declSelector.Sel.Name == "TB"
	isSafeFunc = selector.Sel.Name == "Context"

	return isTestingPackage && isValidType && isSafeFunc
}

func isWithinLoop(exp ast.Expr, node ast.Node, pass *analysis.Pass) bool {
	lhs := getRootIdent(pass, exp)
	if lhs == nil {
		return false
	}

	obj := pass.TypesInfo.ObjectOf(lhs)
	if obj == nil {
		return false
	}

	scope := obj.Parent()
	if scope == nil {
		return false
	}

	return scope.Pos() >= node.Pos() && scope.End() <= node.End()
}

func getRootIdent(pass *analysis.Pass, node ast.Node) *ast.Ident {
	for {
		switch typedNode := node.(type) {
		case *ast.Ident:
			return typedNode
		case *ast.IndexExpr:
			node = typedNode.X
		case *ast.SelectorExpr:
			if sel, ok := pass.TypesInfo.Selections[typedNode]; ok && sel.Indirect() {
				return nil // indirected (pointer) roots don't imply a (safe) copy
			}

			node = typedNode.X
		default:
			return nil
		}
	}
}

func isPointer(pass *analysis.Pass, exp ast.Node) bool {
	switch n := exp.(type) { //nolint:gocritic // Future-proofing with switch instead of if.
	case *ast.SelectorExpr:
		sel, ok := pass.TypesInfo.Selections[n]

		return ok && sel.Indirect()
	}

	return false
}
