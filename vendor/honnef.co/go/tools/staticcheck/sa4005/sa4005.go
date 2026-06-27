package sa4005

import (
	"fmt"
	"go/types"

	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA4005",
		Run:      run,
		Requires: []*analysis.Analyzer{buildir.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:    `Field assignment that will never be observed. Did you mean to use a pointer receiver?`,
		Since:    "2021.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	// The analysis only considers the receiver and its first level
	// fields. It doesn't look at other parameters, nor at nested
	// fields.
	//
	// The analysis does not detect all kinds of dead stores, only
	// those of fields that are never read after the write. That is,
	// we do not flag 'a.x = 1; a.x = 2; _ = a.x'. We might explore
	// this again if we add support for SROA to go/ir and implement
	// https://github.com/dominikh/go-tools/issues/191.

	irpkg := pass.ResultOf[buildir.Analyzer].(*buildir.IR)
fnLoop:
	for _, fn := range irpkg.SrcFuncs {
		if recv := fn.Signature.Recv(); recv == nil {
			continue
		} else if _, ok := recv.Type().Underlying().(*types.Struct); !ok {
			continue
		}

		recv := fn.Params[0]
		refs := irutil.FilterDebug(*recv.Referrers())
		if len(refs) != 1 {
			continue
		}
		store, ok := refs[0].(*ir.Store)
		if !ok {
			continue
		}
		alloc, ok := store.Addr.(*ir.Alloc)
		if !ok || alloc.Heap {
			continue
		}

		reads := map[int][]ir.Instruction{}
		writes := map[int][]ir.Instruction{}
		for _, ref := range *alloc.Referrers() {
			switch ref := ref.(type) {
			case *ir.FieldAddr:
				for _, refref := range *ref.Referrers() {
					switch refref.(type) {
					case *ir.Store:
						writes[ref.Field] = append(writes[ref.Field], refref)
					case *ir.Load:
						reads[ref.Field] = append(reads[ref.Field], refref)
					case *ir.DebugRef:
						continue
					default:
						// this should be safe… if the field address
						// escapes, then alloc.Heap will be true.
						// there should be no instructions left that,
						// given this FieldAddr, without escaping, can
						// effect a load or store.
						continue
					}
				}
			case *ir.Store:
				// we could treat this as a store to every field, but
				// we don't want to decide the semantics of partial
				// struct initializers. should `v = t{x: 1}` also mark
				// v.y as being written to?
				if ref != store {
					continue fnLoop
				}
			case *ir.Load:
				// a load of the entire struct loads every field
				for i := 0; i < recv.Type().Underlying().(*types.Struct).NumFields(); i++ {
					reads[i] = append(reads[i], ref)
				}
			case *ir.DebugRef:
				continue
			default:
				continue fnLoop
			}
		}

		offset := func(instr ir.Instruction) int {
			for i, other := range instr.Block().Instrs {
				if instr == other {
					return i
				}
			}
			panic("couldn't find instruction in its block")
		}

		for field, ws := range writes {
			rs := reads[field]
		wLoop:
			for _, w := range ws {
				for _, r := range rs {
					if w.Block() == r.Block() {
						if offset(r) > offset(w) {
							// found a reachable read of our write
							continue wLoop
						}
					} else if irutil.Reachable(w.Block(), r.Block()) {
						// found a reachable read of our write
						continue wLoop
					}
				}
				fieldName := recv.Type().Underlying().(*types.Struct).Field(field).Name()
				report.Report(pass, w, fmt.Sprintf("ineffective assignment to field %s.%s", recv.Type().(interface{ Obj() *types.TypeName }).Obj().Name(), fieldName))
			}
		}
	}
	return nil, nil
}
