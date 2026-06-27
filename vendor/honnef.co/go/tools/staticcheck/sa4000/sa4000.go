package sa4000

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/types/typeutil"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/edge"
	"golang.org/x/tools/go/ast/inspector"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA4000",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:    `Binary operator has identical expressions on both sides`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	var isFloat func(T types.Type) bool
	isFloat = func(T types.Type) bool {
		tset := typeutil.NewTypeSet(T)
		if len(tset.Terms) == 0 {
			// no terms, so floats are a possibility
			return true
		}
		return tset.Any(func(term *types.Term) bool {
			switch typ := term.Type().Underlying().(type) {
			case *types.Basic:
				kind := typ.Kind()
				return kind == types.Float32 || kind == types.Float64
			case *types.Array:
				return isFloat(typ.Elem())
			case *types.Struct:
				for field := range typ.Fields() {
					if isFloat(field.Type()) {
						return true
					}
				}
				return false
			default:
				return false
			}
		})
	}

	// TODO(dh): this check ignores the existence of side-effects and
	// happily flags fn() == fn() – so far, we've had only two complains
	// about false positives, and it's caught several bugs in real
	// code.
	//
	// We special case functions from the math/rand package. Someone ran
	// into the following false positive: "rand.Intn(2) - rand.Intn(2), which I wrote to generate values {-1, 0, 1} with {0.25, 0.5, 0.25} probability."

	skipComparableCheck := func(c inspector.Cursor) bool {
		op, ok := c.Node().(*ast.BinaryExpr)
		if !ok {
			return false
		}
		if clit, ok := op.X.(*ast.CompositeLit); !ok || len(clit.Elts) != 0 {
			return false
		}
		if clit, ok := op.Y.(*ast.CompositeLit); !ok || len(clit.Elts) != 0 {
			return false
		}

		// TODO(dh): we should probably skip ParenExprs, but users should
		// probably not use unnecessary ParenExprs.
		vspec, ok := c.Parent().Node().(*ast.ValueSpec)
		if !ok {
			return false
		}
		e, i := c.ParentEdge()
		if e != edge.ValueSpec_Values {
			return false
		}
		if vspec.Names[i].Name == "_" {
			// `var _ = T{} == T{}` is permitted, as a compile-time
			// check that T implements comparable.
			return true
		}
		return false
	}

	for c := range code.Cursor(pass).Preorder((*ast.BinaryExpr)(nil)) {
		node := c.Node()
		op := node.(*ast.BinaryExpr)
		switch op.Op {
		case token.EQL, token.NEQ:
			if skipComparableCheck(c) {
				continue
			}
		case token.SUB, token.QUO, token.AND, token.REM, token.OR, token.XOR, token.AND_NOT,
			token.LAND, token.LOR, token.LSS, token.GTR, token.LEQ, token.GEQ:
		default:
			// For some ops, such as + and *, it can make sense to
			// have identical operands
			continue
		}

		if isFloat(pass.TypesInfo.TypeOf(op.X)) {
			// 'float <op> float' makes sense for several operators.
			// We've tried keeping an exact list of operators to allow, but floats keep surprising us. Let's just give up instead.
			continue
		}

		if reflect.TypeOf(op.X) != reflect.TypeOf(op.Y) {
			continue
		}
		if report.Render(pass, op.X) != report.Render(pass, op.Y) {
			continue
		}
		l1, ok1 := op.X.(*ast.BasicLit)
		l2, ok2 := op.Y.(*ast.BasicLit)
		if ok1 && ok2 && l1.Kind == token.INT && l2.Kind == l1.Kind && l1.Value == "0" && l2.Value == l1.Value && code.IsGenerated(pass, l1.Pos()) {
			// cgo generates the following function call:
			// _cgoCheckPointer(_cgoBase0, 0 == 0) – it uses 0 == 0
			// instead of true in case the user shadowed the
			// identifier. Ideally we'd restrict this exception to
			// calls of _cgoCheckPointer, but it's not worth the
			// hassle of keeping track of the stack. <lit> <op> <lit>
			// are very rare to begin with, and we're mostly checking
			// for them to catch typos such as 1 == 1 where the user
			// meant to type i == 1. The odds of a false negative for
			// 0 == 0 are slim.
			continue
		}

		if expr, ok := op.X.(*ast.CallExpr); ok {
			call := code.CallName(pass, expr)
			switch call {
			case "math/rand.Int",
				"math/rand.Int31",
				"math/rand.Int31n",
				"math/rand.Int63",
				"math/rand.Int63n",
				"math/rand.Intn",
				"math/rand.Uint32",
				"math/rand.Uint64",
				"math/rand.ExpFloat64",
				"math/rand.Float32",
				"math/rand.Float64",
				"math/rand.NormFloat64",
				"(*math/rand.Rand).Int",
				"(*math/rand.Rand).Int31",
				"(*math/rand.Rand).Int31n",
				"(*math/rand.Rand).Int63",
				"(*math/rand.Rand).Int63n",
				"(*math/rand.Rand).Intn",
				"(*math/rand.Rand).Uint32",
				"(*math/rand.Rand).Uint64",
				"(*math/rand.Rand).ExpFloat64",
				"(*math/rand.Rand).Float32",
				"(*math/rand.Rand).Float64",
				"(*math/rand.Rand).NormFloat64",
				"math/rand/v2.Int",
				"math/rand/v2.Int32",
				"math/rand/v2.Int32N",
				"math/rand/v2.Int64",
				"math/rand/v2.Int64N",
				"math/rand/v2.IntN",
				"math/rand/v2.N",
				"math/rand/v2.Uint",
				"math/rand/v2.Uint32",
				"math/rand/v2.Uint32N",
				"math/rand/v2.Uint64",
				"math/rand/v2.Uint64N",
				"math/rand/v2.UintN",
				"math/rand/v2.ExpFloat64",
				"math/rand/v2.Float32",
				"math/rand/v2.Float64",
				"math/rand/v2.NormFloat64",
				"(*math/rand/v2.Rand).Int",
				"(*math/rand/v2.Rand).Int32",
				"(*math/rand/v2.Rand).Int32N",
				"(*math/rand/v2.Rand).Int64",
				"(*math/rand/v2.Rand).Int64N",
				"(*math/rand/v2.Rand).IntN",
				"(*math/rand/v2.Rand).N",
				"(*math/rand/v2.Rand).Uint",
				"(*math/rand/v2.Rand).Uint32",
				"(*math/rand/v2.Rand).Uint32N",
				"(*math/rand/v2.Rand).Uint64",
				"(*math/rand/v2.Rand).Uint64N",
				"(*math/rand/v2.Rand).UintN",
				"(*math/rand/v2.Rand).ExpFloat64",
				"(*math/rand/v2.Rand).Float32",
				"(*math/rand/v2.Rand).Float64",
				"(*math/rand/v2.Rand).NormFloat64":
				continue
			}
		}

		report.Report(pass, op, fmt.Sprintf("identical expressions on the left and right side of the '%s' operator", op.Op))
	}

	return nil, nil
}
