package nilerr

import (
	"github.com/gostaticanalysis/nilerr"

	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New() *goanalysis.Linter {
	return goanalysis.
		NewLinterFromAnalyzer(nilerr.Analyzer).
		WithDesc("Find the code that returns nil even if it checks that the error is not nil.").
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
