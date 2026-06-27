package ginkgolinter

import (
	"flag"
	"fmt"

	"golang.org/x/tools/go/analysis"

	"github.com/nunnatsa/ginkgolinter/config"
	"github.com/nunnatsa/ginkgolinter/linter"
	"github.com/nunnatsa/ginkgolinter/version"
)

// NewAnalyzerWithConfig returns an Analyzer.
func NewAnalyzerWithConfig(config *config.Config) *analysis.Analyzer {
	theLinter := linter.NewGinkgoLinter(config)

	return &analysis.Analyzer{
		Name: "ginkgolinter",
		Doc:  fmt.Sprintf(doc, version.Version()),
		Run:  theLinter.Run,
	}
}

// NewAnalyzer returns an Analyzer - the package interface with nogo
func NewAnalyzer() *analysis.Analyzer {
	config := &config.Config{
		SuppressLen:               false,
		SuppressNil:               false,
		SuppressErr:               false,
		SuppressCompare:           false,
		ForbidFocus:               false,
		AllowHaveLen0:             false,
		ForceExpectTo:             false,
		ForceSucceedForFuncs:      false,
		ForceAssertionDescription: false,
	}

	a := NewAnalyzerWithConfig(config)

	a.Flags.Init("ginkgolinter", flag.ExitOnError)
	a.Flags.BoolVar(&config.SuppressLen, "suppress-len-assertion", config.SuppressLen, "Suppress warning for wrong length assertions")
	a.Flags.BoolVar(&config.SuppressNil, "suppress-nil-assertion", config.SuppressNil, "Suppress warning for wrong nil assertions")
	a.Flags.BoolVar(&config.SuppressErr, "suppress-err-assertion", config.SuppressErr, "Suppress warning for wrong error assertions")
	a.Flags.BoolVar(&config.SuppressCompare, "suppress-compare-assertion", config.SuppressCompare, "Suppress warning for wrong comparison assertions")
	a.Flags.BoolVar(&config.SuppressAsync, "suppress-async-assertion", config.SuppressAsync, "Suppress warning for function call in async assertion, like Eventually")
	a.Flags.BoolVar(&config.ValidateAsyncIntervals, "validate-async-intervals", config.ValidateAsyncIntervals, "best effort validation of async intervals (timeout and polling); ignored the suppress-async-assertion flag is true")
	a.Flags.BoolVar(&config.SuppressTypeCompare, "suppress-type-compare-assertion", config.SuppressTypeCompare, "Suppress warning for comparing values from different types, like int32 and uint32")
	a.Flags.BoolVar(&config.AllowHaveLen0, "allow-havelen-0", config.AllowHaveLen0, "Do not warn for HaveLen(0); default = false")
	a.Flags.BoolVar(&config.ForceExpectTo, "force-expect-to", config.ForceExpectTo, "force using `Expect` with `To`, `ToNot` or `NotTo`. reject using `Expect` with `Should` or `ShouldNot`; default = false (not forced)")
	a.Flags.BoolVar(&config.ForbidFocus, "forbid-focus-container", config.ForbidFocus, "trigger a warning for ginkgo focus containers like FDescribe, FContext, FWhen or FIt; default = false.")
	a.Flags.BoolVar(&config.ForbidSpecPollution, "forbid-spec-pollution", config.ForbidSpecPollution, "trigger a warning for variable assignments in ginkgo containers like Describe, Context and When, instead of in BeforeEach(); default = false.")
	a.Flags.BoolVar(&config.ForceSucceedForFuncs, "force-succeed", config.ForceSucceedForFuncs, "force using the Succeed matcher for error functions, and the HaveOccurred matcher for non-function error values")
	a.Flags.BoolVar(&config.ForceAssertionDescription, "force-assertion-description", config.ForceAssertionDescription, "force adding assertion descriptions to gomega matchers; default = false")
	a.Flags.BoolVar(&config.ForeToNot, "force-tonot", config.ForeToNot, "force using `ToNot`, `ShouldNot` instead of To(Not()); default = false")

	return a
}

// Analyzer is the interface to go_vet
var Analyzer = NewAnalyzer()
