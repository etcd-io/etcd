package sa5012

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:      "SA5012",
		Run:       run,
		FactTypes: []analysis.Fact{new(evenElements)},
		Requires:  []*analysis.Analyzer{buildir.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: "Passing odd-sized slice to function expecting even size",
		Text: `Some functions that take slices as parameters expect the slices to have an even number of elements. 
Often, these functions treat elements in a slice as pairs. 
For example, \'strings.NewReplacer\' takes pairs of old and new strings, 
and calling it with an odd number of elements would be an error.`,
		Since:    "2020.2",
		Severity: lint.SeverityError,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

type evenElements struct{}

func (evenElements) AFact() {}

func (evenElements) String() string { return "needs even elements" }

func findSliceLength(v ir.Value) int {
	// TODO(dh): VRP would help here

	v = irutil.Flatten(v)
	val := func(v ir.Value) int {
		if v, ok := v.(*ir.Const); ok {
			return int(v.Int64())
		}
		return -1
	}
	switch v := v.(type) {
	case *ir.Slice:
		low := 0
		high := -1
		if v.Low != nil {
			low = val(v.Low)
		}
		if v.High != nil {
			high = val(v.High)
		} else {
			switch vv := v.X.(type) {
			case *ir.Alloc:
				high = int(typeutil.Dereference(vv.Type()).Underlying().(*types.Array).Len())
			case *ir.Slice:
				high = findSliceLength(vv)
			}
		}
		if low == -1 || high == -1 {
			return -1
		}
		return high - low
	default:
		return -1
	}
}

func flagSliceLens(pass *analysis.Pass) {
	var tag evenElements

	for _, fn := range pass.ResultOf[buildir.Analyzer].(*buildir.IR).SrcFuncs {
		for _, b := range fn.Blocks {
			for _, instr := range b.Instrs {
				call, ok := instr.(ir.CallInstruction)
				if !ok {
					continue
				}
				callee := call.Common().StaticCallee()
				if callee == nil {
					continue
				}
				for argi, arg := range call.Common().Args {
					if callee.Signature.Recv() != nil {
						if argi == 0 {
							continue
						}
						argi--
					}

					_, ok := arg.Type().Underlying().(*types.Slice)
					if !ok {
						continue
					}
					param := callee.Signature.Params().At(argi)
					if !pass.ImportObjectFact(param, &tag) {
						continue
					}

					// TODO handle stubs

					// we know the argument has to have even length.
					// now let's try to find its length
					if n := findSliceLength(arg); n > -1 && n%2 != 0 {
						src := call.Source().(*ast.CallExpr).Args[argi]
						sig := call.Common().Signature()
						var label string
						if argi == sig.Params().Len()-1 && sig.Variadic() {
							label = "variadic argument"
						} else {
							label = "argument"
						}
						// Note that param.Name() is guaranteed to not
						// be empty, otherwise the function couldn't
						// have enforced its length.
						report.Report(pass, src, fmt.Sprintf("%s %q is expected to have even number of elements, but has %d elements", label, param.Name(), n))
					}
				}
			}
		}
	}
}

func findSliceLenChecks(pass *analysis.Pass) {
	// mark all function parameters that have to be of even length
	for _, fn := range pass.ResultOf[buildir.Analyzer].(*buildir.IR).SrcFuncs {
		for _, b := range fn.Blocks {
			// all paths go through this block
			if !b.Dominates(fn.Exit) {
				continue
			}

			// if foo % 2 != 0
			ifi, ok := b.Control().(*ir.If)
			if !ok {
				continue
			}
			cmp, ok := ifi.Cond.(*ir.BinOp)
			if !ok {
				continue
			}
			var needle uint64
			switch cmp.Op {
			case token.NEQ:
				// look for != 0
				needle = 0
			case token.EQL:
				// look for == 1
				needle = 1
			default:
				continue
			}

			rem, ok1 := cmp.X.(*ir.BinOp)
			k, ok2 := cmp.Y.(*ir.Const)
			if ok1 != ok2 {
				continue
			}
			if !ok1 {
				rem, ok1 = cmp.Y.(*ir.BinOp)
				k, ok2 = cmp.X.(*ir.Const)
			}
			if !ok1 || !ok2 || rem.Op != token.REM || k.Value.Kind() != constant.Int || k.Uint64() != needle {
				continue
			}
			k, ok = rem.Y.(*ir.Const)
			if !ok || k.Value.Kind() != constant.Int || k.Uint64() != 2 {
				continue
			}

			// if len(foo) % 2 != 0
			call, ok := rem.X.(*ir.Call)
			if !ok || !irutil.IsCallTo(call.Common(), "len") {
				continue
			}

			// we're checking the length of a parameter that is a slice
			// TODO(dh): support parameters that have flown through sigmas and phis
			param, ok := call.Call.Args[0].(*ir.Parameter)
			if !ok {
				continue
			}
			if !typeutil.All(param.Type(), typeutil.IsSlice) {
				continue
			}

			// if len(foo) % 2 != 0 then panic
			if _, ok := b.Succs[0].Control().(*ir.Panic); !ok {
				continue
			}

			pass.ExportObjectFact(param.Object(), new(evenElements))
		}
	}
}

func findIndirectSliceLenChecks(pass *analysis.Pass) {
	seen := map[*ir.Function]struct{}{}

	var doFunction func(fn *ir.Function)
	doFunction = func(fn *ir.Function) {
		if _, ok := seen[fn]; ok {
			return
		}
		seen[fn] = struct{}{}

		for _, b := range fn.Blocks {
			// all paths go through this block
			if !b.Dominates(fn.Exit) {
				continue
			}

			for _, instr := range b.Instrs {
				call, ok := instr.(*ir.Call)
				if !ok {
					continue
				}
				callee := call.Call.StaticCallee()
				if callee == nil {
					continue
				}

				if callee.Pkg == fn.Pkg || callee.Pkg == nil {
					doFunction(callee)
				}

				for argi, arg := range call.Call.Args {
					if callee.Signature.Recv() != nil {
						if argi == 0 {
							continue
						}
						argi--
					}

					// TODO(dh): support parameters that have flown through length-preserving instructions
					param, ok := arg.(*ir.Parameter)
					if !ok {
						continue
					}
					if !typeutil.All(param.Type(), typeutil.IsSlice) {
						continue
					}

					// We can't use callee.Params to look up the
					// parameter, because Params is not populated for
					// external functions. In our modular analysis.
					// any function in any package that isn't the
					// current package is considered "external", as it
					// has been loaded from export data only.
					sigParams := callee.Signature.Params()

					if !pass.ImportObjectFact(sigParams.At(argi), new(evenElements)) {
						continue
					}
					pass.ExportObjectFact(param.Object(), new(evenElements))
				}
			}
		}
	}

	for _, fn := range pass.ResultOf[buildir.Analyzer].(*buildir.IR).SrcFuncs {
		doFunction(fn)
	}
}

func run(pass *analysis.Pass) (any, error) {
	findSliceLenChecks(pass)
	findIndirectSliceLenChecks(pass)
	flagSliceLens(pass)

	return nil, nil
}
