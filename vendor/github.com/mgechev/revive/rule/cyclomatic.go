package rule

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/mgechev/revive/lint"
)

// Based on https://github.com/fzipp/gocyclo

// CyclomaticRule sets restriction for maximum cyclomatic complexity.
type CyclomaticRule struct {
	maxComplexity int
}

const defaultMaxCyclomaticComplexity = 10

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *CyclomaticRule) Configure(arguments lint.Arguments) error {
	if len(arguments) < 1 {
		r.maxComplexity = defaultMaxCyclomaticComplexity
		return nil
	}

	complexity, ok := arguments[0].(int64) // Alt. non panicking version
	if !ok {
		return fmt.Errorf("invalid argument for cyclomatic complexity; expected int but got %T", arguments[0])
	}
	r.maxComplexity = int(complexity)
	return nil
}

// Apply applies the rule to given file.
func (r *CyclomaticRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure
	for _, decl := range file.AST.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		c := complexity(fn)
		if c > r.maxComplexity {
			failures = append(failures, lint.Failure{
				Confidence: 1,
				Category:   lint.FailureCategoryMaintenance,
				Failure: fmt.Sprintf("function %s has cyclomatic complexity %d (> max enabled %d)",
					funcName(fn), c, r.maxComplexity),
				Node: fn,
			})
		}
	}

	return failures
}

// Name returns the rule name.
func (*CyclomaticRule) Name() string {
	return "cyclomatic"
}

// funcName returns the name representation of a function or method:
// "(Type).Name" for methods or simply "Name" for functions.
func funcName(fn *ast.FuncDecl) string {
	declarationHasReceiver := fn.Recv != nil && fn.Recv.NumFields() > 0
	if declarationHasReceiver {
		typ := fn.Recv.List[0].Type
		return fmt.Sprintf("(%s).%s", recvString(typ), fn.Name)
	}

	return fn.Name.Name
}

// recvString returns a string representation of recv of the
// form "T", "*T", or "BADRECV" (if not a proper receiver type).
func recvString(recv ast.Expr) string {
	switch t := recv.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + recvString(t.X)
	}
	return "BADRECV"
}

// complexity calculates the cyclomatic complexity of a function.
func complexity(fn *ast.FuncDecl) int {
	v := complexityVisitor{}
	ast.Walk(&v, fn)
	return v.Complexity
}

type complexityVisitor struct {
	// Complexity is the cyclomatic complexity
	Complexity int
}

// Visit implements the [ast.Visitor] interface.
func (v *complexityVisitor) Visit(n ast.Node) ast.Visitor {
	switch n := n.(type) {
	case *ast.FuncDecl, *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
		v.Complexity++
	case *ast.BinaryExpr:
		if n.Op == token.LAND || n.Op == token.LOR {
			v.Complexity++
		}
	}
	return v
}
