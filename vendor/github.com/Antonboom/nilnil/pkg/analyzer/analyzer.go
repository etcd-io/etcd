package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	name = "nilnil"
	doc  = "Checks that there is no simultaneous return of `nil` error and an invalid value."

	nilNilReportMsg       = "return both a `nil` error and an invalid value: use a sentinel error instead"
	notNilNotNilReportMsg = "return both a non-nil error and a valid value: use separate returns instead"
)

// New returns new nilnil analyzer.
func New() *analysis.Analyzer {
	n := newNilNil()

	a := &analysis.Analyzer{
		Name:     name,
		Doc:      doc,
		Run:      n.run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}
	a.Flags.Var(&n.checkedTypes, "checked-types", "comma separated list of return types to check")
	a.Flags.BoolVar(&n.detectOpposite, "detect-opposite", false,
		"in addition, to detect opposite situation (simultaneous return of non-nil error and valid value)")
	a.Flags.BoolVar(&n.onlyTwo, "only-two", true,
		"to check functions with only two return values")

	return a
}

type nilNil struct {
	checkedTypes   checkedTypes
	detectOpposite bool
	onlyTwo        bool
}

func newNilNil() *nilNil {
	return &nilNil{
		checkedTypes:   newDefaultCheckedTypes(),
		detectOpposite: false,
		onlyTwo:        true,
	}
}

var funcAndReturns = []ast.Node{
	(*ast.FuncDecl)(nil),
	(*ast.FuncLit)(nil),
	(*ast.ReturnStmt)(nil),
}

func (n *nilNil) run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	var fs funcTypeStack
	insp.Nodes(funcAndReturns, func(node ast.Node, push bool) (proceed bool) {
		switch v := node.(type) {
		case *ast.FuncLit:
			if push {
				fs.Push(v.Type)
			} else {
				fs.Pop()
			}

		case *ast.FuncDecl:
			if push {
				fs.Push(v.Type)
			} else {
				fs.Pop()
			}

		case *ast.ReturnStmt:
			ft := fs.Top() // Current function.

			if !push {
				return false
			}
			if len(v.Results) < 2 {
				return false
			}
			if (ft == nil) || (ft.Results == nil) || (len(ft.Results.List) != len(v.Results)) {
				// Unreachable.
				return false
			}

			lastIdx := len(ft.Results.List) - 1
			if n.onlyTwo {
				lastIdx = 1
			}

			lastFtRes := ft.Results.List[lastIdx]
			if !implementsError(pass.TypesInfo.TypeOf(lastFtRes.Type)) {
				return false
			}

			retErr := v.Results[lastIdx]
			for i := range lastIdx {
				retVal := v.Results[i]

				zv, ok := n.isDangerNilType(pass.TypesInfo.TypeOf(ft.Results.List[i].Type))
				if !ok {
					continue
				}

				if ((zv == zeroValueNil) && isNil(pass, retVal) && isNil(pass, retErr)) ||
					((zv == zeroValueZero) && isZero(retVal) && isNil(pass, retErr)) {
					pass.Reportf(v.Pos(), nilNilReportMsg)
					return false
				}

				if n.detectOpposite && (((zv == zeroValueNil) && !isNil(pass, retVal) && !isNil(pass, retErr)) ||
					((zv == zeroValueZero) && !isZero(retVal) && !isNil(pass, retErr))) {
					pass.Reportf(v.Pos(), notNilNotNilReportMsg)
					return false
				}
			}
		}

		return true
	})

	return nil, nil //nolint:nilnil // Integration interface of analysis.Analyzer.
}

type zeroValue int

const (
	zeroValueNil = iota + 1
	zeroValueZero
)

func (n *nilNil) isDangerNilType(t types.Type) (zeroValue, bool) {
	switch v := types.Unalias(t).(type) {
	case *types.Pointer:
		return zeroValueNil, n.checkedTypes.Contains(ptrType)

	case *types.Signature:
		return zeroValueNil, n.checkedTypes.Contains(funcType)

	case *types.Interface:
		return zeroValueNil, n.checkedTypes.Contains(ifaceType)

	case *types.Map:
		return zeroValueNil, n.checkedTypes.Contains(mapType)

	case *types.Chan:
		return zeroValueNil, n.checkedTypes.Contains(chanType)

	case *types.Basic:
		if v.Kind() == types.Uintptr {
			return zeroValueZero, n.checkedTypes.Contains(uintptrType)
		}
		if v.Kind() == types.UnsafePointer {
			return zeroValueNil, n.checkedTypes.Contains(unsafeptrType)
		}

	case *types.Named:
		return n.isDangerNilType(v.Underlying())
	}
	return 0, false
}

var errorIface = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

func implementsError(t types.Type) bool {
	_, ok := t.Underlying().(*types.Interface)
	return ok && types.Implements(t, errorIface)
}

func isNil(pass *analysis.Pass, e ast.Expr) bool {
	i, ok := e.(*ast.Ident)
	if !ok {
		return false
	}

	_, ok = pass.TypesInfo.ObjectOf(i).(*types.Nil)
	return ok
}

func isZero(e ast.Expr) bool {
	bl, ok := e.(*ast.BasicLit)
	if !ok {
		return false
	}
	if bl.Kind != token.INT {
		return false
	}

	v, err := strconv.ParseInt(bl.Value, 0, 64)
	if err != nil {
		return false
	}
	return v == 0
}
