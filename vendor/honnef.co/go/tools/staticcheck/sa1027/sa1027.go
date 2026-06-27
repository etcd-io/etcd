package sa1027

import (
	"fmt"
	"go/types"

	"honnef.co/go/tools/analysis/callcheck"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1027",
		Requires: []*analysis.Analyzer{buildir.Analyzer},
		Run:      callcheck.Analyzer(checkAtomicAlignment),
	},
	Doc: &lint.RawDocumentation{
		Title: `Atomic access to 64-bit variable must be 64-bit aligned`,
		Text: `On ARM, x86-32, and 32-bit MIPS, it is the caller's responsibility to
arrange for 64-bit alignment of 64-bit words accessed atomically. The
first word in a variable or in an allocated struct, array, or slice
can be relied upon to be 64-bit aligned.

You can use the structlayout tool to inspect the alignment of fields
in a struct.`,
		Since:    "2019.2",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var checkAtomicAlignment = map[string]callcheck.Check{
	"sync/atomic.AddInt64":             checkAtomicAlignmentImpl,
	"sync/atomic.AddUint64":            checkAtomicAlignmentImpl,
	"sync/atomic.CompareAndSwapInt64":  checkAtomicAlignmentImpl,
	"sync/atomic.CompareAndSwapUint64": checkAtomicAlignmentImpl,
	"sync/atomic.LoadInt64":            checkAtomicAlignmentImpl,
	"sync/atomic.LoadUint64":           checkAtomicAlignmentImpl,
	"sync/atomic.StoreInt64":           checkAtomicAlignmentImpl,
	"sync/atomic.StoreUint64":          checkAtomicAlignmentImpl,
	"sync/atomic.SwapInt64":            checkAtomicAlignmentImpl,
	"sync/atomic.SwapUint64":           checkAtomicAlignmentImpl,
}

func checkAtomicAlignmentImpl(call *callcheck.Call) {
	sizes := call.Pass.TypesSizes
	if sizes.Sizeof(types.Typ[types.Uintptr]) != 4 {
		// Not running on a 32-bit platform
		return
	}
	v, ok := irutil.Flatten(call.Args[0].Value.Value).(*ir.FieldAddr)
	if !ok {
		// TODO(dh): also check indexing into arrays and slices
		return
	}
	T := v.X.Type().Underlying().(*types.Pointer).Elem().Underlying().(*types.Struct)
	fields := make([]*types.Var, 0, T.NumFields())
	for i := 0; i < T.NumFields() && i <= v.Field; i++ {
		fields = append(fields, T.Field(i))
	}

	off := sizes.Offsetsof(fields)[v.Field]
	if off%8 != 0 {
		msg := fmt.Sprintf("address of non 64-bit aligned field %s passed to %s",
			T.Field(v.Field).Name(),
			irutil.CallName(call.Instr.Common()))
		call.Invalid(msg)
	}
}
