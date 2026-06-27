package purity

// TODO(dh): we should split this into two facts, one tracking actual purity, and one tracking side-effects. A function
// that returns a heap allocation isn't pure, but it may be free of side effects.

import (
	"go/types"
	"reflect"

	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

type IsPure struct{}

func (*IsPure) AFact()           {}
func (d *IsPure) String() string { return "is pure" }

type Result map[*types.Func]*IsPure

var Analyzer = &analysis.Analyzer{
	Name:       "fact_purity",
	Doc:        "Mark pure functions",
	Run:        purity,
	Requires:   []*analysis.Analyzer{buildir.Analyzer},
	FactTypes:  []analysis.Fact{(*IsPure)(nil)},
	ResultType: reflect.TypeFor[Result](),
}

var pureStdlib = map[string]struct{}{
	"errors.New":                      {},
	"fmt.Errorf":                      {},
	"fmt.Sprintf":                     {},
	"fmt.Sprint":                      {},
	"sort.Reverse":                    {},
	"strings.Map":                     {},
	"strings.Repeat":                  {},
	"strings.Replace":                 {},
	"strings.Title":                   {},
	"strings.ToLower":                 {},
	"strings.ToLowerSpecial":          {},
	"strings.ToTitle":                 {},
	"strings.ToTitleSpecial":          {},
	"strings.ToUpper":                 {},
	"strings.ToUpperSpecial":          {},
	"strings.Trim":                    {},
	"strings.TrimFunc":                {},
	"strings.TrimLeft":                {},
	"strings.TrimLeftFunc":            {},
	"strings.TrimPrefix":              {},
	"strings.TrimRight":               {},
	"strings.TrimRightFunc":           {},
	"strings.TrimSpace":               {},
	"strings.TrimSuffix":              {},
	"(*net/http.Request).WithContext": {},
	"time.Now":                        {},
	"time.Parse":                      {},
	"time.ParseInLocation":            {},
	"time.Unix":                       {},
	"time.UnixMicro":                  {},
	"time.UnixMilli":                  {},
	"(time.Time).Add":                 {},
	"(time.Time).AddDate":             {},
	"(time.Time).After":               {},
	"(time.Time).Before":              {},
	"(time.Time).Clock":               {},
	"(time.Time).Compare":             {},
	"(time.Time).Date":                {},
	"(time.Time).Day":                 {},
	"(time.Time).Equal":               {},
	"(time.Time).Format":              {},
	"(time.Time).GoString":            {},
	"(time.Time).GobEncode":           {},
	"(time.Time).Hour":                {},
	"(time.Time).ISOWeek":             {},
	"(time.Time).In":                  {},
	"(time.Time).IsDST":               {},
	"(time.Time).IsZero":              {},
	"(time.Time).Local":               {},
	"(time.Time).Location":            {},
	"(time.Time).MarshalBinary":       {},
	"(time.Time).MarshalJSON":         {},
	"(time.Time).MarshalText":         {},
	"(time.Time).Minute":              {},
	"(time.Time).Month":               {},
	"(time.Time).Nanosecond":          {},
	"(time.Time).Round":               {},
	"(time.Time).Second":              {},
	"(time.Time).String":              {},
	"(time.Time).Sub":                 {},
	"(time.Time).Truncate":            {},
	"(time.Time).UTC":                 {},
	"(time.Time).Unix":                {},
	"(time.Time).UnixMicro":           {},
	"(time.Time).UnixMilli":           {},
	"(time.Time).UnixNano":            {},
	"(time.Time).Weekday":             {},
	"(time.Time).Year":                {},
	"(time.Time).YearDay":             {},
	"(time.Time).Zone":                {},
	"(time.Time).ZoneBounds":          {},
}

func purity(pass *analysis.Pass) (any, error) {
	seen := map[*ir.Function]struct{}{}
	irpkg := pass.ResultOf[buildir.Analyzer].(*buildir.IR).Pkg
	var check func(fn *ir.Function) (ret bool)
	check = func(fn *ir.Function) (ret bool) {
		if fn.Object() == nil {
			// TODO(dh): support closures
			return false
		}
		if pass.ImportObjectFact(fn.Object(), new(IsPure)) {
			return true
		}
		if fn.Pkg != irpkg {
			// Function is in another package but wasn't marked as
			// pure, ergo it isn't pure
			return false
		}
		// Break recursion
		if _, ok := seen[fn]; ok {
			return false
		}

		seen[fn] = struct{}{}
		defer func() {
			if ret {
				pass.ExportObjectFact(fn.Object(), &IsPure{})
			}
		}()

		if irutil.IsStub(fn) {
			return false
		}

		if _, ok := pureStdlib[fn.Object().(*types.Func).FullName()]; ok {
			return true
		}

		if fn.Signature.Results().Len() == 0 {
			// A function with no return values is empty or is doing some
			// work we cannot see (for example because of build tags);
			// don't consider it pure.
			return false
		}

		var isBasic func(typ types.Type) bool
		isBasic = func(typ types.Type) bool {
			switch u := typ.Underlying().(type) {
			case *types.Basic:
				return true
			case *types.Struct:
				for field := range u.Fields() {
					if !isBasic(field.Type()) {
						return false
					}
				}
				return true
			default:
				return false
			}
		}

		for _, param := range fn.Params {
			// TODO(dh): this may not be strictly correct. pure code can, to an extent, operate on non-basic types.
			if !isBasic(param.Type()) {
				return false
			}
		}

		// Don't consider external functions pure.
		if fn.Blocks == nil {
			return false
		}
		checkCall := func(common *ir.CallCommon) bool {
			if common.IsInvoke() {
				return false
			}
			builtin, ok := common.Value.(*ir.Builtin)
			if !ok {
				if common.StaticCallee() != fn {
					if common.StaticCallee() == nil {
						return false
					}
					if !check(common.StaticCallee()) {
						return false
					}
				}
			} else {
				switch builtin.Name() {
				case "len", "cap":
				default:
					return false
				}
			}
			return true
		}

		var isStackAddr func(ir.Value) bool
		isStackAddr = func(v ir.Value) bool {
			switch v := v.(type) {
			case *ir.Alloc:
				return !v.Heap
			case *ir.FieldAddr:
				return isStackAddr(v.X)
			default:
				return false
			}
		}
		for _, b := range fn.Blocks {
			for _, ins := range b.Instrs {
				switch ins := ins.(type) {
				case *ir.Call:
					if !checkCall(ins.Common()) {
						return false
					}
				case *ir.Defer:
					if !checkCall(&ins.Call) {
						return false
					}
				case *ir.Select:
					return false
				case *ir.Send:
					return false
				case *ir.Go:
					return false
				case *ir.Panic:
					return false
				case *ir.Store:
					if !isStackAddr(ins.Addr) {
						return false
					}
				case *ir.FieldAddr:
					if !isStackAddr(ins.X) {
						return false
					}
				case *ir.Alloc:
					// TODO(dh): make use of proper escape analysis
					if ins.Heap {
						return false
					}
				case *ir.Load:
					if !isStackAddr(ins.X) {
						return false
					}
				}
			}
		}
		return true
	}
	for _, fn := range pass.ResultOf[buildir.Analyzer].(*buildir.IR).SrcFuncs {
		check(fn)
	}

	out := Result{}
	for _, fact := range pass.AllObjectFacts() {
		out[fact.Object.(*types.Func)] = fact.Fact.(*IsPure)
	}
	return out, nil
}
