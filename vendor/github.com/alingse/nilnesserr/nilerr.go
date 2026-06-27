// some code was copy from https://github.com/gostaticanalysis/nilerr/blob/master/nilerr.go

package nilnesserr

import (
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ssa"
)

var errType = types.Universe.Lookup("error").Type().Underlying().(*types.Interface) // nolint: forcetypeassert

func isErrType(res ssa.Value) bool {
	return types.Implements(res.Type(), errType)
}

func isConstNil(res ssa.Value) bool {
	v, ok := res.(*ssa.Const)
	if ok && v.IsNil() {
		return true
	}

	return false
}

func extractCheckedErrorValue(binOp *ssa.BinOp) ssa.Value {
	if isErrType(binOp.X) && isConstNil(binOp.Y) {
		return binOp.X
	}
	if isErrType(binOp.Y) && isConstNil(binOp.X) {
		return binOp.Y
	}

	return nil
}

type errFact fact

func findLastNonnilValue(errors []errFact, res ssa.Value) ssa.Value {
	if len(errors) == 0 {
		return nil
	}

	for j := len(errors) - 1; j >= 0; j-- {
		last := errors[j]
		if last.value == res {
			return nil
		} else if last.nilness == isnonnil {
			return last.value
		}
	}

	return nil
}

func checkNilnesserr(pass *analysis.Pass, b *ssa.BasicBlock, errors []errFact, isNilnees func(value ssa.Value) bool) {
	for _, instr := range b.Instrs {
		pos := instr.Pos()
		if !pos.IsValid() {
			continue
		}

		switch instr := instr.(type) {
		case *ssa.Return:
			for _, value := range instr.Results {
				if checkSSAValue(value, errors, isNilnees) {
					pass.Report(analysis.Diagnostic{
						Pos:     pos,
						Message: linterReturnMessage,
					})
				}
			}
		case *ssa.Call:
			for _, value := range instr.Call.Args {
				if checkSSAValue(value, errors, isNilnees) {
					pass.Report(analysis.Diagnostic{
						Pos:     pos,
						Message: linterCallMessage,
					})
				}
			}

			// extra check for variadic arguments
			variadicArgs := checkVariadicCall(instr)
			for _, value := range variadicArgs {
				if checkSSAValue(value, errors, isNilnees) {
					pass.Report(analysis.Diagnostic{
						Pos:     pos,
						Message: linterVariadicCallMessage,
					})
				}
			}
		}
	}
}

func checkSSAValue(res ssa.Value, errors []errFact, isNilnees func(value ssa.Value) bool) bool {
	if !isErrType(res) || isConstNil(res) || !isNilnees(res) {
		return false
	}
	// check the lastValue error that is isnonnil
	lastValue := findLastNonnilValue(errors, res)

	return lastValue != nil
}

func checkVariadicCall(call *ssa.Call) []ssa.Value {
	alloc := validateVariadicCall(call)
	if alloc == nil {
		return nil
	}

	return extractVariadicErrors(alloc)
}

/*
example: fmt.Errorf("call Do2 got err %w", err)

type *ssa.Alloc instr new [1]any (varargs)
type *ssa.IndexAddr instr &t4[0:int]
type *ssa.ChangeInterface instr change interface any <- error (t0)
type *ssa.Store instr *t5 = t6
...
type *ssa.Slice instr slice t4[:]
type *ssa.Call instr fmt.Errorf("call Do2 got err ...":string, t7...)
*/
func validateVariadicCall(call *ssa.Call) *ssa.Alloc {
	fn, ok := call.Call.Value.(*ssa.Function)
	if !ok {
		return nil
	}
	if !fn.Signature.Variadic() {
		return nil
	}

	if len(call.Call.Args) == 0 {
		return nil
	}
	lastArg := call.Call.Args[len(call.Call.Args)-1]
	slice, ok := lastArg.(*ssa.Slice)
	if !ok {
		return nil
	}
	// check is t[:]
	if !(slice.Low == nil && slice.High == nil && slice.Max == nil) {
		return nil
	}
	alloc, ok := slice.X.(*ssa.Alloc)
	if !ok {
		return nil
	}
	valueType, ok := alloc.Type().(*types.Pointer)
	if !ok {
		return nil
	}

	// check is array
	_, ok = valueType.Elem().(*types.Array)
	if !ok {
		return nil
	}

	return alloc
}

// the Referrer chain is like this.
// Alloc --> IndexAddr --> ChangeInterface --> Store ---> Slice.
// Alloc --> IndexAddr --> Store --> Slice.
func extractVariadicErrors(alloc *ssa.Alloc) []ssa.Value {
	values := make([]ssa.Value, 0)

	for _, instr := range *alloc.Referrers() {
		indexAddr, ok := instr.(*ssa.IndexAddr)
		if !ok {
			continue
		}
		for _, instr2 := range *indexAddr.Referrers() {
			store, ok := instr2.(*ssa.Store)
			if !ok {
				continue
			}
			value := store.Val
			if change, ok := value.(*ssa.ChangeInterface); ok {
				value = change.X
			}
			values = append(values, value)
		}
	}

	return values
}
