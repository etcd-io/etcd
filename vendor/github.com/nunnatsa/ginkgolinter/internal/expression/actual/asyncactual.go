package actual

import (
	"go/ast"
	gotypes "go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/nunnatsa/ginkgolinter/internal/intervals"
)

type AsyncArg struct {
	valid bool
	fun   *ast.CallExpr

	timeoutInterval intervals.DurationValue
	pollingInterval intervals.DurationValue
	tooManyTimeouts bool
	tooManyPolling  bool
}

func newAsyncArg(origExpr, cloneExpr, orig, clone *ast.CallExpr, argType gotypes.Type, pass *analysis.Pass, actualOffset int, timePkg string) *AsyncArg {
	var (
		fun     *ast.CallExpr
		valid   = true
		timeout intervals.DurationValue
		polling intervals.DurationValue
	)

	if _, isActualFuncCall := orig.Args[actualOffset].(*ast.CallExpr); isActualFuncCall {
		fun = clone.Args[actualOffset].(*ast.CallExpr)
		valid = isValidAsyncValueType(argType)
	}

	timeoutOffset := actualOffset + 1
	//var err error
	tooManyTimeouts := false
	tooManyPolling := false

	if len(orig.Args) > timeoutOffset {
		timeout = intervals.GetDuration(pass, timeoutOffset, orig.Args[timeoutOffset], clone.Args[timeoutOffset], timePkg)
		pollingOffset := actualOffset + 2
		if len(orig.Args) > pollingOffset {
			polling = intervals.GetDuration(pass, pollingOffset, orig.Args[pollingOffset], clone.Args[pollingOffset], timePkg)
		}
	}
	selOrig := origExpr.Fun.(*ast.SelectorExpr)
	selClone := cloneExpr.Fun.(*ast.SelectorExpr)

	for {
		callOrig, ok := selOrig.X.(*ast.CallExpr)
		if !ok {
			break
		}
		callClone := selClone.X.(*ast.CallExpr)

		funOrig, ok := callOrig.Fun.(*ast.SelectorExpr)
		if !ok {
			break
		}
		funClone := callClone.Fun.(*ast.SelectorExpr)

		switch funOrig.Sel.Name {
		case "WithTimeout", "Within":
			if timeout != nil {
				tooManyTimeouts = true
			} else if len(callOrig.Args) == 1 {
				timeout = intervals.GetDurationFromValue(pass, callOrig.Args[0], callClone.Args[0])
			}

		case "WithPolling", "ProbeEvery":
			if polling != nil {
				tooManyPolling = true
			} else if len(callOrig.Args) == 1 {
				polling = intervals.GetDurationFromValue(pass, callOrig.Args[0], callClone.Args[0])
			}
		}

		selOrig = funOrig
		selClone = funClone
	}

	return &AsyncArg{
		valid:           valid,
		fun:             fun,
		timeoutInterval: timeout,
		pollingInterval: polling,
		tooManyTimeouts: tooManyTimeouts,
		tooManyPolling:  tooManyPolling,
	}
}

func (a *AsyncArg) IsValid() bool {
	return a.valid
}

func (a *AsyncArg) Timeout() intervals.DurationValue {
	return a.timeoutInterval
}

func (a *AsyncArg) Polling() intervals.DurationValue {
	return a.pollingInterval
}

func (a *AsyncArg) TooManyTimeouts() bool {
	return a.tooManyTimeouts
}

func (a *AsyncArg) TooManyPolling() bool {
	return a.tooManyPolling
}

func isValidAsyncValueType(t gotypes.Type) bool {
	switch t.(type) {
	// allow functions that return function or channel.
	case *gotypes.Signature, *gotypes.Chan, *gotypes.Pointer:
		return true
	case *gotypes.Named:
		return isValidAsyncValueType(t.Underlying())
	}

	return false
}
