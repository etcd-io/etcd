package sa4023

import (
	"fmt"
	"go/token"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/nilness"
	"honnef.co/go/tools/analysis/facts/typedness"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/exp/typeparams"
	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA4023",
		Run:      run,
		Requires: []*analysis.Analyzer{buildir.Analyzer, typedness.Analysis, nilness.Analysis},
	},
	Doc: &lint.RawDocumentation{
		Title: `Impossible comparison of interface value with untyped nil`,
		Text: `Under the covers, interfaces are implemented as two elements, a
type T and a value V. V is a concrete value such as an int,
struct or pointer, never an interface itself, and has type T. For
instance, if we store the int value 3 in an interface, the
resulting interface value has, schematically, (T=int, V=3). The
value V is also known as the interface's dynamic value, since a
given interface variable might hold different values V (and
corresponding types T) during the execution of the program.

An interface value is nil only if the V and T are both
unset, (T=nil, V is not set), In particular, a nil interface will
always hold a nil type. If we store a nil pointer of type *int
inside an interface value, the inner type will be *int regardless
of the value of the pointer: (T=*int, V=nil). Such an interface
value will therefore be non-nil even when the pointer value V
inside is nil.

This situation can be confusing, and arises when a nil value is
stored inside an interface value such as an error return:

    func returnsError() error {
        var p *MyError = nil
        if bad() {
            p = ErrBad
        }
        return p // Will always return a non-nil error.
    }

If all goes well, the function returns a nil p, so the return
value is an error interface value holding (T=*MyError, V=nil).
This means that if the caller compares the returned error to nil,
it will always look as if there was an error even if nothing bad
happened. To return a proper nil error to the caller, the
function must return an explicit nil:

    func returnsError() error {
        if bad() {
            return ErrBad
        }
        return nil
    }

It's a good idea for functions that return errors always to use
the error type in their signature (as we did above) rather than a
concrete type such as \'*MyError\', to help guarantee the error is
created correctly. As an example, \'os.Open\' returns an error even
though, if not nil, it's always of concrete type *os.PathError.

Similar situations to those described here can arise whenever
interfaces are used. Just keep in mind that if any concrete value
has been stored in the interface, the interface will not be nil.
For more information, see The Laws of
Reflection at https://golang.org/doc/articles/laws_of_reflection.html.

This text has been copied from
https://golang.org/doc/faq#nil_error, licensed under the Creative
Commons Attribution 3.0 License.`,
		Since:    "2020.2",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny, // TODO should this be MergeIfAll?
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	// The comparison 'fn() == nil' can never be true if fn() returns
	// an interface value and only returns typed nils. This is usually
	// a mistake in the function itself, but all we can say for
	// certain is that the comparison is pointless.
	//
	// Flag results if no untyped nils are being returned, but either
	// known typed nils, or typed unknown nilness are being returned.

	irpkg := pass.ResultOf[buildir.Analyzer].(*buildir.IR)
	typedness := pass.ResultOf[typedness.Analysis].(*typedness.Result)
	nilness := pass.ResultOf[nilness.Analysis].(*nilness.Result)
	for _, fn := range irpkg.SrcFuncs {
		for _, b := range fn.Blocks {
			for _, instr := range b.Instrs {
				binop, ok := instr.(*ir.BinOp)
				if !ok || !(binop.Op == token.EQL || binop.Op == token.NEQ) {
					continue
				}
				if _, ok := binop.X.Type().Underlying().(*types.Interface); !ok || typeparams.IsTypeParam(binop.X.Type()) {
					// TODO support swapped X and Y
					continue
				}

				k, ok := binop.Y.(*ir.Const)
				if !ok || !k.IsNil() {
					// if binop.X is an interface, then binop.Y can
					// only be a Const if its untyped. A typed nil
					// constant would first be passed to
					// MakeInterface.
					continue
				}

				var idx int
				var obj *types.Func
				switch x := irutil.Flatten(binop.X).(type) {
				case *ir.Call:
					callee := x.Call.StaticCallee()
					if callee == nil {
						continue
					}
					obj, _ = callee.Object().(*types.Func)
					idx = 0
				case *ir.Extract:
					call, ok := irutil.Flatten(x.Tuple).(*ir.Call)
					if !ok {
						continue
					}
					callee := call.Call.StaticCallee()
					if callee == nil {
						continue
					}
					obj, _ = callee.Object().(*types.Func)
					idx = x.Index
				case *ir.MakeInterface:
					var qualifier string
					switch binop.Op {
					case token.EQL:
						qualifier = "never"
					case token.NEQ:
						qualifier = "always"
					default:
						panic("unreachable")
					}

					terms, err := typeparams.NormalTerms(x.X.Type())
					if len(terms) == 0 || err != nil {
						// Type is a type parameter with no type terms (or we couldn't determine the terms). Such a type
						// _can_ be nil when put in an interface value.
						continue
					}

					if report.HasRange(x.X) {
						report.Report(pass, binop, fmt.Sprintf("this comparison is %s true", qualifier),
							report.Related(x.X, "the lhs of the comparison gets its value from here and has a concrete type"))
					} else {
						// we can't generate related information for this, so make the diagnostic itself slightly more useful
						report.Report(pass, binop, fmt.Sprintf("this comparison is %s true; the lhs of the comparison has been assigned a concretely typed value", qualifier))
					}
					continue
				}
				if obj == nil {
					continue
				}

				isNil, onlyGlobal := nilness.MayReturnNil(obj, idx)
				if typedness.MustReturnTyped(obj, idx) && isNil && !onlyGlobal && !code.IsInTest(pass, binop) {
					// Don't flag these comparisons in tests. Tests
					// may be explicitly enforcing the invariant that
					// a value isn't nil.

					var qualifier string
					switch binop.Op {
					case token.EQL:
						qualifier = "never"
					case token.NEQ:
						qualifier = "always"
					default:
						panic("unreachable")
					}
					report.Report(pass, binop, fmt.Sprintf("this comparison is %s true", qualifier),
						// TODO support swapped X and Y
						report.Related(binop.X, fmt.Sprintf("the lhs of the comparison is the %s return value of this function call", report.Ordinal(idx+1))),
						report.Related(obj, fmt.Sprintf("%s never returns a nil interface value", typeutil.FuncName(obj))))
				}
			}
		}
	}

	return nil, nil
}
