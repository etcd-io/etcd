package rules

import (
	"github.com/nunnatsa/ginkgolinter/config"
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
)

const valueInEventually = "use a function call in %[1]s. This actually checks nothing, because %[1]s receives the function returned value, instead of function itself, and this value is never changed"

// AsyncFuncCallRule checks that there is no function call actual parameter, in an async actual method (e.g. Eventually).
//
// Async actual methods should get the function itself, not a function call, because when you pass a function call,
// the function is executed immediately and Eventually receives the static return value, making it a synchronous
// operation that checks the same value repeatedly instead of re-evaluating the function.
//
// The rule allows functions that return a function, a channel or a pointer.
//
// Example:
//
//	// Bad:
//	Eventually(someFunction()).Should(Equal(5))
//
//	// Good:
//	Eventually(someFunction).Should(Equal(5))
type AsyncFuncCallRule struct{}

func (r AsyncFuncCallRule) isApplied(gexp *expression.GomegaExpression, config config.Config) bool {
	if config.SuppressAsync || !gexp.IsAsync() {
		return false
	}

	if asyncArg := gexp.GetAsyncActualArg(); asyncArg != nil {
		return !asyncArg.IsValid()
	}

	return false
}

func (r AsyncFuncCallRule) Apply(gexp *expression.GomegaExpression, config config.Config, reportBuilder *reports.Builder) bool {
	if r.isApplied(gexp, config) {
		gexp.AppendWithArgsToActual()

		reportBuilder.AddIssue(true, valueInEventually, gexp.GetActualFuncName())
	}
	return false
}
