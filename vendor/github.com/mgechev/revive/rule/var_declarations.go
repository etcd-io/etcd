package rule

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

var zeroLiteral = map[string]bool{
	"false": true, // bool
	// runes
	`'\x00'`: true,
	`'\000'`: true,
	// strings
	`""`: true,
	"``": true,
	// numerics
	"0":   true,
	"0.":  true,
	"0.0": true,
	"0i":  true,
}

// VarDeclarationsRule reduces redundancies around variable declaration.
type VarDeclarationsRule struct{}

// Apply applies the rule to given file.
func (*VarDeclarationsRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	fileAst := file.AST
	walker := &lintVarDeclarations{
		file:    file,
		fileAst: fileAst,
		onFailure: func(failure lint.Failure) {
			failures = append(failures, failure)
		},
	}

	file.Pkg.TypeCheck()
	ast.Walk(walker, fileAst)

	return failures
}

// Name returns the rule name.
func (*VarDeclarationsRule) Name() string {
	return "var-declaration"
}

type lintVarDeclarations struct {
	fileAst   *ast.File
	file      *lint.File
	lastGen   *ast.GenDecl
	onFailure func(lint.Failure)
}

func (w *lintVarDeclarations) Visit(node ast.Node) ast.Visitor {
	switch v := node.(type) {
	case *ast.GenDecl:
		isVarOrConstDeclaration := v.Tok == token.CONST || v.Tok == token.VAR
		if !isVarOrConstDeclaration {
			return nil
		}
		w.lastGen = v
		return w
	case *ast.ValueSpec:
		isConstDeclaration := w.lastGen.Tok == token.CONST
		if isConstDeclaration {
			return nil
		}
		if len(v.Names) > 1 || v.Type == nil || len(v.Values) == 0 {
			return nil
		}
		rhs := v.Values[0]
		// An underscore var appears in a common idiom for compile-time interface satisfaction,
		// as in "var _ Interface = (*Concrete)(nil)".
		if astutils.IsIdent(v.Names[0], "_") {
			return nil
		}
		// If the RHS is a isZero value, suggest dropping it.
		isZero := false
		if lit, ok := rhs.(*ast.BasicLit); ok {
			isZero = isZeroValue(lit.Value, v.Type)
		} else if astutils.IsIdent(rhs, "nil") {
			isZero = true
		}
		if isZero {
			w.onFailure(lint.Failure{
				Confidence: 0.9,
				Node:       rhs,
				Category:   lint.FailureCategoryZeroValue,
				Failure:    fmt.Sprintf("should drop = %s from declaration of var %s; it is the zero value", w.file.Render(rhs), v.Names[0]),
			})
			return nil
		}
		lhsTyp := w.file.Pkg.TypeOf(v.Type)
		rhsTyp := w.file.Pkg.TypeOf(rhs)

		if !validType(lhsTyp) || !validType(rhsTyp) {
			// Type checking failed (often due to missing imports).
			return nil
		}

		if !types.Identical(lhsTyp, rhsTyp) {
			// Assignment to a different type is not redundant.
			return nil
		}

		// The next three conditions are for suppressing the warning in situations
		// where we were unable to typecheck.

		// If the LHS type is an interface, don't warn, since it is probably a
		// concrete type on the RHS. Note that our feeble lexical check here
		// will only pick up interface{} and other literal interface types;
		// that covers most of the cases we care to exclude right now.
		if _, ok := v.Type.(*ast.InterfaceType); ok {
			return nil
		}
		// If the RHS is an untyped const, only warn if the LHS type is its default type.
		if defType, ok := w.file.IsUntypedConst(rhs); ok && !astutils.IsIdent(v.Type, defType) {
			return nil
		}

		w.onFailure(lint.Failure{
			Category:   lint.FailureCategoryTypeInference,
			Confidence: 0.8,
			Node:       v.Type,
			Failure:    fmt.Sprintf("should omit type %s from declaration of var %s; it will be inferred from the right-hand side", w.file.Render(v.Type), v.Names[0]),
		})
		return nil
	}
	return w
}

func validType(t types.Type) bool {
	return t != nil &&
		t != types.Typ[types.Invalid] &&
		!strings.Contains(t.String(), "invalid type") // good but not foolproof
}

func isZeroValue(litValue string, typ ast.Expr) bool {
	switch val := typ.(type) {
	case *ast.Ident:
		if val.Name == "any" {
			return litValue == "nil"
		}
	case *ast.InterfaceType:
		return litValue == "nil"
	}

	return zeroLiteral[litValue]
}
