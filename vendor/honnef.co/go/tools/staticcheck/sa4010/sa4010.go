package sa4010

import (
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA4010",
		Run:      run,
		Requires: []*analysis.Analyzer{buildir.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:    `The result of \'append\' will never be observed anywhere`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAll,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	isAppend := func(ins ir.Value) bool {
		call, ok := ins.(*ir.Call)
		if !ok {
			return false
		}
		if call.Call.IsInvoke() {
			return false
		}
		if builtin, ok := call.Call.Value.(*ir.Builtin); !ok || builtin.Name() != "append" {
			return false
		}
		return true
	}

	// We have to be careful about aliasing.
	// Multiple slices may refer to the same backing array,
	// making appends observable even when we don't see the result of append be used anywhere.
	//
	// We will have to restrict ourselves to slices that have been allocated within the function,
	// haven't been sliced,
	// and haven't been passed anywhere that could retain them (such as function calls or memory stores).
	//
	// We check whether an append should be flagged in two steps.
	//
	// In the first step, we look at the data flow graph, starting in reverse from the argument to append, till we reach the root.
	// This graph must only consist of the following instructions:
	//
	// - phi
	// - sigma
	// - slice
	// - const nil
	// - MakeSlice
	// - Alloc
	// - calls to append
	//
	// If this step succeeds, we look at all referrers of the values found in the first step, recursively.
	// These referrers must either be in the set of values found in the first step,
	// be DebugRefs,
	// or fulfill the same type requirements as step 1, with the exception of appends, which are forbidden.
	//
	// If both steps succeed then we know that the backing array hasn't been aliased in an observable manner.
	//
	// We could relax these restrictions by making use of additional information:
	// - if we passed the slice to a function that doesn't retain the slice then we can still flag it
	// - if a slice has been sliced but is dead afterwards, we can flag appends to the new slice

	// OPT(dh): We could cache the results of both validate functions.
	// However, we only use these functions on values that we otherwise want to flag, which are very few.
	// Not caching values hasn't increased the runtimes for the standard library nor k8s.
	var validateArgument func(v ir.Value, seen map[ir.Value]struct{}) bool
	validateArgument = func(v ir.Value, seen map[ir.Value]struct{}) bool {
		if _, ok := seen[v]; ok {
			// break cycle
			return true
		}
		seen[v] = struct{}{}
		switch v := v.(type) {
		case *ir.Phi:
			for _, edge := range v.Edges {
				if !validateArgument(edge, seen) {
					return false
				}
			}
			return true
		case *ir.Sigma:
			return validateArgument(v.X, seen)
		case *ir.Slice:
			return validateArgument(v.X, seen)
		case *ir.Const:
			return true
		case *ir.MakeSlice:
			return true
		case *ir.Alloc:
			return true
		case *ir.Call:
			if isAppend(v) {
				return validateArgument(v.Call.Args[0], seen)
			}
			return false
		default:
			return false
		}
	}

	var validateReferrers func(v ir.Value, seen map[ir.Instruction]struct{}) bool
	validateReferrers = func(v ir.Value, seen map[ir.Instruction]struct{}) bool {
		for _, ref := range *v.Referrers() {
			if _, ok := seen[ref]; ok {
				continue
			}

			seen[ref] = struct{}{}
			switch ref.(type) {
			case *ir.Phi:
			case *ir.Sigma:
			case *ir.Slice:
			case *ir.Const:
			case *ir.MakeSlice:
			case *ir.Alloc:
			case *ir.DebugRef:
			default:
				return false
			}

			if ref, ok := ref.(ir.Value); ok {
				if !validateReferrers(ref, seen) {
					return false
				}
			}
		}
		return true
	}

	for _, fn := range pass.ResultOf[buildir.Analyzer].(*buildir.IR).SrcFuncs {
		for _, block := range fn.Blocks {
			for _, ins := range block.Instrs {
				val, ok := ins.(ir.Value)
				if !ok || !isAppend(val) {
					continue
				}

				isUsed := false
				visited := map[ir.Instruction]bool{}
				var walkRefs func(refs []ir.Instruction)
				walkRefs = func(refs []ir.Instruction) {
				loop:
					for _, ref := range refs {
						if visited[ref] {
							continue
						}
						visited[ref] = true
						if _, ok := ref.(*ir.DebugRef); ok {
							continue
						}
						switch ref := ref.(type) {
						case *ir.Phi:
							walkRefs(*ref.Referrers())
						case *ir.Sigma:
							walkRefs(*ref.Referrers())
						case ir.Value:
							if !isAppend(ref) {
								isUsed = true
							} else {
								walkRefs(*ref.Referrers())
							}
						case ir.Instruction:
							isUsed = true
							break loop
						}
					}
				}

				refs := val.Referrers()
				if refs == nil {
					continue
				}
				walkRefs(*refs)

				if isUsed {
					continue
				}

				seen := map[ir.Value]struct{}{}
				if !validateArgument(ins.(*ir.Call).Call.Args[0], seen) {
					continue
				}

				seen2 := map[ir.Instruction]struct{}{}
				for k := range seen {
					// the only values we allow are also instructions, so this type assertion cannot fail
					seen2[k.(ir.Instruction)] = struct{}{}
				}
				seen2[ins] = struct{}{}
				failed := false
				for v := range seen {
					if !validateReferrers(v, seen2) {
						failed = true
						break
					}
				}
				if !failed {
					report.Report(pass, ins, "this result of append is never used, except maybe in other appends")
				}
			}
		}
	}
	return nil, nil
}
