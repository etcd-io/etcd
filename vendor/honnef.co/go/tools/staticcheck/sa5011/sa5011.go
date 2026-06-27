package sa5011

import (
	"go/types"

	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA5011",
		Run:      run,
		Requires: []*analysis.Analyzer{buildir.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Possible nil pointer dereference`,

		Text: `A pointer is being dereferenced unconditionally, while
also being checked against nil in another place. This suggests that
the pointer may be nil and dereferencing it may panic. This is
commonly a result of improperly ordered code or missing return
statements. Consider the following examples:

    func fn(x *int) {
        fmt.Println(*x)

        // This nil check is equally important for the previous dereference
        if x != nil {
            foo(*x)
        }
    }

    func TestFoo(t *testing.T) {
        x := compute()
        if x == nil {
            t.Errorf("nil pointer received")
        }

        // t.Errorf does not abort the test, so if x is nil, the next line will panic.
        foo(*x)
    }

Staticcheck tries to deduce which functions abort control flow.
For example, it is aware that a function will not continue
execution after a call to \'panic\' or \'log.Fatal\'. However, sometimes
this detection fails, in particular in the presence of
conditionals. Consider the following example:

    func Log(msg string, level int) {
        fmt.Println(msg)
        if level == levelFatal {
            os.Exit(1)
        }
    }

    func Fatal(msg string) {
        Log(msg, levelFatal)
    }

    func fn(x *int) {
        if x == nil {
            Fatal("unexpected nil pointer")
        }
        fmt.Println(*x)
    }

Staticcheck will flag the dereference of \'x\', even though it is perfectly
safe. Staticcheck is not able to deduce that a call to
Fatal will exit the program. For the time being, the easiest
workaround is to modify the definition of Fatal like so:

    func Fatal(msg string) {
        Log(msg, levelFatal)
        panic("unreachable")
    }

We also hard-code functions from common logging packages such as
logrus. Please file an issue if we're missing support for a
popular package.`,
		Since:    "2020.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	// This is an extremely trivial check that doesn't try to reason
	// about control flow. That is, phis and sigmas do not propagate
	// any information. As such, we can flag this:
	//
	// 	_ = *x
	// 	if x == nil { return }
	//
	// but we cannot flag this:
	//
	// 	if x == nil { println(x) }
	// 	_ = *x
	//
	// but we can flag this, because the if's body doesn't use x:
	//
	// 	if x == nil { println("this is bad") }
	//  _ = *x
	//
	// nor many other variations of conditional uses of or assignments to x.
	//
	// However, even this trivial implementation finds plenty of
	// real-world bugs, such as dereference before nil pointer check,
	// or using t.Error instead of t.Fatal when encountering nil
	// pointers.
	//
	// On the flip side, our naive implementation avoids false positives in branches, such as
	//
	// 	if x != nil { _ = *x }
	//
	// due to the same lack of propagating information through sigma
	// nodes. x inside the branch will be independent of the x in the
	// nil pointer check.
	//
	//
	// We could implement a more powerful check, but then we'd be
	// getting false positives instead of false negatives because
	// we're incapable of deducing relationships between variables.
	// For example, a function might return a pointer and an error,
	// and the error being nil guarantees that the pointer is not nil.
	// Depending on the surrounding code, the pointer may still end up
	// being checked against nil in one place, and guarded by a check
	// on the error in another, which would lead to us marking some
	// loads as unsafe.
	//
	// Unfortunately, simply hard-coding the relationship between
	// return values wouldn't eliminate all false positives, either.
	// Many other more subtle relationships exist. An abridged example
	// from real code:
	//
	// if a == nil && b == nil { return }
	// c := fn(a)
	// if c != "" { _ = *a }
	//
	// where `fn` is guaranteed to return a non-empty string if a
	// isn't nil.
	//
	// We choose to err on the side of false negatives.

	isNilConst := func(v ir.Value) bool {
		if typeutil.IsPointerLike(v.Type()) {
			if k, ok := v.(*ir.Const); ok {
				return k.IsNil()
			}
		}
		return false
	}

	for _, fn := range pass.ResultOf[buildir.Analyzer].(*buildir.IR).SrcFuncs {
		maybeNil := map[ir.Value]ir.Instruction{}
		for _, b := range fn.Blocks {
			for _, instr := range b.Instrs {
				// Originally we looked at all ir.BinOp, but that would lead to calls like 'assert(x != nil)' causing false positives.
				// Restrict ourselves to actual if statements, as these are more likely to affect control flow in a way we can observe.
				if instr, ok := instr.(*ir.If); ok {
					if cond, ok := instr.Cond.(*ir.BinOp); ok {
						if isNilConst(cond.X) {
							maybeNil[cond.Y] = cond
						}
						if isNilConst(cond.Y) {
							maybeNil[cond.X] = cond
						}
					}
				}
			}
		}

		for _, b := range fn.Blocks {
			for _, instr := range b.Instrs {
				var ptr ir.Value
				switch instr := instr.(type) {
				case *ir.Load:
					ptr = instr.X
				case *ir.Store:
					ptr = instr.Addr
				case *ir.IndexAddr:
					ptr = instr.X
					if typeutil.All(ptr.Type(), func(term *types.Term) bool {
						if _, ok := term.Type().Underlying().(*types.Slice); ok {
							return true
						}
						return false
					}) {
						// indexing a nil slice does not cause a nil pointer panic
						//
						// Note: This also works around the bad lowering of range loops over slices
						// (https://github.com/dominikh/go-tools/issues/1053)
						continue
					}
				case *ir.FieldAddr:
					ptr = instr.X
				}
				if ptr != nil {
					switch ptr.(type) {
					case *ir.Alloc, *ir.FieldAddr, *ir.IndexAddr:
						// these cannot be nil
						continue
					}
					if r, ok := maybeNil[ptr]; ok {
						report.Report(pass, instr, "possible nil pointer dereference",
							report.Related(r, "this check suggests that the pointer can be nil"))
					}
				}
			}
		}
	}

	return nil, nil
}
