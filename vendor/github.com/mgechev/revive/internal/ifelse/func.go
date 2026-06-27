package ifelse

import (
	"fmt"
	"go/ast"
)

// Call contains the name of a function that deviates control flow.
type Call struct {
	Pkg  string // The package qualifier of the function, if not built-in.
	Name string // The function name.
}

// DeviatingFuncs lists known control flow deviating function calls.
var DeviatingFuncs = map[Call]BranchKind{
	{"os", "Exit"}:     Exit,
	{"log", "Fatal"}:   Exit,
	{"log", "Fatalf"}:  Exit,
	{"log", "Fatalln"}: Exit,
	{"", "panic"}:      Panic,
	{"log", "Panic"}:   Panic,
	{"log", "Panicf"}:  Panic,
	{"log", "Panicln"}: Panic,
}

// ExprCall gets the [Call] of an [ast.ExprStmt], if any.
func ExprCall(expr *ast.ExprStmt) (Call, bool) {
	call, ok := expr.X.(*ast.CallExpr)
	if !ok {
		return Call{}, false
	}
	switch v := call.Fun.(type) {
	case *ast.Ident:
		return Call{Name: v.Name}, true
	case *ast.SelectorExpr:
		if ident, ok := v.X.(*ast.Ident); ok {
			return Call{Name: v.Sel.Name, Pkg: ident.Name}, true
		}
	}
	return Call{}, false
}

// String returns the function name with package qualifier (if any).
func (f Call) String() string {
	if f.Pkg != "" {
		return fmt.Sprintf("%s.%s", f.Pkg, f.Name)
	}
	return f.Name
}
