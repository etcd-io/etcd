package canonicalheader

import (
	"fmt"
	"go/ast"
	"go/types"
	"net/http"

	"github.com/go-toolsmith/astcast"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

const (
	pkgPath = "net/http"
	name    = "Header"
)

//nolint:gochecknoglobals // struct is not big, can be skip.
var Analyzer = &analysis.Analyzer{
	Name:     "canonicalheader",
	Doc:      "canonicalheader checks whether net/http.Header uses canonical header",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

type argumenter interface {
	diagnostic(canonicalHeader string) analysis.Diagnostic
	value() string
}

func run(pass *analysis.Pass) (any, error) {
	var headerObject types.Object
	for _, object := range pass.TypesInfo.Uses {
		if object.Pkg() != nil &&
			object.Pkg().Path() == pkgPath &&
			object.Name() == name {
			headerObject = object
			break
		}
	}

	if headerObject == nil {
		//nolint:nilnil // nothing to do here, because http.Header{} not usage.
		return nil, nil
	}

	spctor, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, fmt.Errorf("want %T, got %T", spctor, pass.ResultOf[inspect.Analyzer])
	}

	wellKnownHeaders := initialism()

	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}
	var outerErr error

	spctor.Preorder(nodeFilter, func(n ast.Node) {
		if outerErr != nil {
			return
		}

		callExp, ok := n.(*ast.CallExpr)
		if !ok {
			return
		}

		var (
			// gotType type receiver.
			gotType       types.Type
			gotMethodName string
		)

		switch t := typeutil.Callee(pass.TypesInfo, callExp).(type) {
		// Direct call method.
		case *types.Func:
			fn := t
			// Find net/http.Header{} by function call.
			signature, ok := fn.Type().(*types.Signature)
			if !ok {
				return
			}

			recv := signature.Recv()

			// It's a func, not a method.
			if recv == nil {
				return
			}
			gotType = recv.Type()
			gotMethodName = astcast.ToSelectorExpr(callExp.Fun).Sel.Name

		// h := http.Header{}
		// f := h.Get
		// v("Test-Value").
		case *types.Var:
			ident, ok := callExp.Fun.(*ast.Ident)
			if !ok {
				return
			}

			if ident.Obj == nil {
				return
			}

			// f := h.Get.
			assign, ok := ident.Obj.Decl.(*ast.AssignStmt)
			if !ok {
				return
			}

			// For case `i, v := 0, h.Get`.
			// indexAssign--^.
			indexAssign := -1
			for i, lh := range assign.Lhs {
				// Find by name of variable.
				if astcast.ToIdent(lh).Name == ident.Name {
					indexAssign = i
				}
			}

			// Not found.
			if indexAssign == -1 {
				return
			}

			if len(assign.Rhs) <= indexAssign {
				return
			}

			sel, ok := assign.Rhs[indexAssign].(*ast.SelectorExpr)
			if !ok {
				return
			}

			gotMethodName = sel.Sel.Name
			ident, ok = sel.X.(*ast.Ident)
			if !ok {
				return
			}

			obj := pass.TypesInfo.ObjectOf(ident)
			gotType = obj.Type()

		default:
			return
		}

		// It is not net/http.Header{}.
		if !types.Identical(gotType, headerObject.Type()) {
			return
		}

		// Search for known methods where the key is the first arg.
		if !isValidMethod(gotMethodName) {
			return
		}

		// Should be more than one. Because get the value by index it
		// will not be superfluous.
		if len(callExp.Args) == 0 {
			return
		}

		callArg := callExp.Args[0]

		// Check for type casting from myString to string.
		// it could be: Get(string(string(string(myString)))).
		// need this node------------------------^^^^^^^^.
		for {
			// If it is not *ast.CallExpr, this is a value.
			c, ok := callArg.(*ast.CallExpr)
			if !ok {
				break
			}

			// Some function is called, skip this case.
			if len(c.Args) == 0 {
				return
			}

			f, ok := c.Fun.(*ast.Ident)
			if !ok {
				break
			}

			obj := pass.TypesInfo.ObjectOf(f)
			// nil may be by code, but not by logic.
			// TypeInfo should contain of type.
			if obj == nil {
				break
			}

			// This is function.
			// Skip this method call.
			_, ok = obj.Type().(*types.Signature)
			if ok {
				return
			}

			callArg = c.Args[0]
		}

		var arg argumenter
		switch t := callArg.(type) {
		case *ast.BasicLit:
			lString, err := newLiteralString(t)
			if err != nil {
				return
			}

			arg = lString

		case *ast.Ident:
			constString, err := newConstantKey(pass.TypesInfo, t)
			if err != nil {
				return
			}

			arg = constString

		default:
			return
		}

		argValue := arg.value()
		headerKeyCanonical := http.CanonicalHeaderKey(argValue)
		if argValue == headerKeyCanonical {
			return
		}

		headerKeyCanonical, isWellKnown := canonicalHeaderKey(argValue, wellKnownHeaders)
		if argValue == headerKeyCanonical || isWellKnown {
			return
		}

		pass.Report(arg.diagnostic(headerKeyCanonical))
	})

	return nil, outerErr
}

func canonicalHeaderKey(s string, m map[string]string) (string, bool) {
	canonical := http.CanonicalHeaderKey(s)

	wellKnown, ok := m[canonical]
	if !ok {
		return canonical, ok
	}

	return wellKnown, ok
}

func isValidMethod(name string) bool {
	switch name {
	case "Get", "Set", "Add", "Del", "Values":
		return true
	default:
		return false
	}
}
