package rule

import (
	"fmt"
	"go/ast"
	"go/types"

	"github.com/mgechev/revive/internal/typeparams"
	"github.com/mgechev/revive/lint"
)

// UnexportedReturnRule warns when a public function returns an unexported type.
type UnexportedReturnRule struct{}

// Apply applies the rule to given file.
func (*UnexportedReturnRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	if !file.IsImportable() {
		// Symbols defined in such files cannot be used in other packages.
		// Therefore, we don't need to check for unexported return types.
		return nil
	}

	var failures []lint.Failure

	file.Pkg.TypeCheck()

	for _, decl := range file.AST.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		if fn.Type.Results == nil {
			continue
		}

		if !fn.Name.IsExported() {
			continue
		}

		thing := "func"
		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			thing = "method"
			if !ast.IsExported(typeparams.ReceiverType(fn)) {
				// Don't report exported methods of unexported types,
				// such as private implementations of sort.Interface.
				continue
			}
		}

		for _, ret := range fn.Type.Results.List {
			typ := file.Pkg.TypeOf(ret.Type)
			if exportedType(typ) {
				continue
			}

			failures = append(failures, lint.Failure{
				Category:   lint.FailureCategoryUnexportedTypeInAPI,
				Node:       ret.Type,
				Confidence: 0.8,
				Failure: fmt.Sprintf("exported %s %s returns unexported type %s, which can be annoying to use",
					thing, fn.Name.Name, typ),
			})

			break // only flag one
		}
	}

	return failures
}

// Name returns the rule name.
func (*UnexportedReturnRule) Name() string {
	return "unexported-return"
}

// exportedType reports whether typ is an exported type.
// It is imprecise, and will err on the side of returning true,
// such as for composite types.
func exportedType(typ types.Type) bool {
	switch t := typ.(type) {
	case *types.Alias:
		obj := t.Obj()
		switch {
		// Builtin types have no package.
		case obj.Pkg() == nil:
		case obj.Exported():
		default:
			_, ok := t.Underlying().(*types.Interface)
			return ok
		}
		return true
	case *types.Named:
		obj := t.Obj()
		switch {
		// Builtin types have no package.
		case obj.Pkg() == nil:
		case obj.Exported():
		default:
			_, ok := t.Underlying().(*types.Interface)
			return ok
		}
		return true
	case *types.Map:
		return exportedType(t.Key()) && exportedType(t.Elem())
	case interface {
		Elem() types.Type
	}: // array, slice, pointer, chan
		return exportedType(t.Elem())
	}
	// Be conservative about other types, such as struct, interface, etc.
	return true
}
