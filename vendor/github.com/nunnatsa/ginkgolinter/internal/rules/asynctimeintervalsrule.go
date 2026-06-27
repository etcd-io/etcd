package rules

import (
	"go/ast"
	"time"

	"github.com/nunnatsa/ginkgolinter/config"
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/intervals"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
)

const (
	multipleTimeouts               = "timeout defined more than once"
	multiplePolling                = "polling defined more than once"
	onlyUseTimeDurationForInterval = "only use time.Duration for timeout and polling in Eventually() or Consistently()"
	pollingGreaterThanTimeout      = "timeout must not be shorter than the polling interval"
)

// AsyncTimeIntervalsRule ensures that the timeout and polling intervals are used correctly in asynchronous assertions.
// It reports issues if:
// - the timeout or polling is defined more than once
//
// Example:
//
//	// Bad:
//	Eventually(someFunction).WithTimeout(1 * time.Second).WithTimeout(10 * time.Second).Should(Equal(5))
//
//	// or
//	Eventually(someFunction, 10).WithTimeout(10 * time.Second).Should(Equal(5))
//
//	// Good:
//	Eventually(someFunction).WithTimeout(10 * time.Second).Should(Equal(5))
//
// - the polling interval is greater than the timeout interval
//
//	// Bad:
//	Eventually(someFunction).WithTimeout(1 * time.Second).WithPolling(10 * time.Second).Should(Equal(5))
//
//	// Good:
//	Eventually(someFunction).WithTimeout(10 * time.Second).WithPolling(1 * time.Second).Should(Equal(5))
//
// - intervals are not using proper time.Duration types (e.g., using raw numbers instead of time.Duration)
//
//	// Bad:
//	Eventually(someFunction, 10).Should(Equal(5))
//
//	// Good:
//	Eventually(someFunction).WithTimeout(10 * time.Second).Should(Equal(5))
type AsyncTimeIntervalsRule struct{}

func (r AsyncTimeIntervalsRule) isApplied(gexp *expression.GomegaExpression, config config.Config) bool {
	return !config.SuppressAsync && config.ValidateAsyncIntervals && gexp.IsAsync()
}

func (r AsyncTimeIntervalsRule) Apply(gexp *expression.GomegaExpression, config config.Config, reportBuilder *reports.Builder) bool {
	if r.isApplied(gexp, config) {
		asyncArg := gexp.GetAsyncActualArg()
		if asyncArg.TooManyTimeouts() {
			reportBuilder.AddIssue(false, multipleTimeouts)
		}

		if asyncArg.TooManyPolling() {
			reportBuilder.AddIssue(false, multiplePolling)
		}

		timeoutDuration := checkInterval(gexp, asyncArg.Timeout(), reportBuilder)
		pollingDuration := checkInterval(gexp, asyncArg.Polling(), reportBuilder)

		if timeoutDuration > 0 && pollingDuration > 0 && pollingDuration > timeoutDuration {
			reportBuilder.AddIssue(false, pollingGreaterThanTimeout)
		}
	}

	return false
}

func checkInterval(gexp *expression.GomegaExpression, durVal intervals.DurationValue, reportBuilder *reports.Builder) time.Duration {
	if durVal != nil {
		switch to := durVal.(type) {
		case *intervals.RealDurationValue, *intervals.UnknownDurationTypeValue:

		case *intervals.NumericDurationValue, *intervals.StringDurationValue:
			if checkNumericInterval(gexp.GetActualClone(), to) {
				reportBuilder.AddIssue(true, onlyUseTimeDurationForInterval)
			}

		case *intervals.UnknownDurationValue:
			reportBuilder.AddIssue(true, onlyUseTimeDurationForInterval)
		}

		return durVal.Duration()
	}

	return 0
}

func checkNumericInterval(intervalMethod *ast.CallExpr, interval intervals.DurationValue) bool {
	if interval != nil {
		if numVal, ok := interval.(intervals.NumericValue); ok {
			if offset := numVal.GetOffset(); offset > 0 {
				intervalMethod.Args[offset] = numVal.GetDurationExpr()
				return true
			}
		}
	}

	return false
}
