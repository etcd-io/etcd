package ginkgolinter

import (
	"github.com/nunnatsa/ginkgolinter"
	glconfig "github.com/nunnatsa/ginkgolinter/config"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.GinkgoLinterSettings) *goanalysis.Linter {
	cfg := &glconfig.Config{}

	if settings != nil {
		cfg = &glconfig.Config{
			SuppressLen:               settings.SuppressLenAssertion,
			SuppressNil:               settings.SuppressNilAssertion,
			SuppressErr:               settings.SuppressErrAssertion,
			SuppressCompare:           settings.SuppressCompareAssertion,
			SuppressAsync:             settings.SuppressAsyncAssertion,
			ForbidFocus:               settings.ForbidFocusContainer,
			SuppressTypeCompare:       settings.SuppressTypeCompareWarning,
			AllowHaveLen0:             settings.AllowHaveLenZero,
			ForceExpectTo:             settings.ForceExpectTo,
			ValidateAsyncIntervals:    settings.ValidateAsyncIntervals,
			ForbidSpecPollution:       settings.ForbidSpecPollution,
			ForceSucceedForFuncs:      settings.ForceSucceedForFuncs,
			ForceAssertionDescription: settings.ForceAssertionDescription,
			ForeToNot:                 settings.ForeToNot,
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(ginkgolinter.NewAnalyzerWithConfig(cfg)).
		WithDesc("enforces standards of using ginkgo and gomega").
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
