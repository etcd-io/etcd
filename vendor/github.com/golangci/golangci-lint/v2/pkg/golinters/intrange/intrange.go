package intrange

import (
	"github.com/ckaznocha/intrange"

	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New() *goanalysis.Linter {
	return goanalysis.
		NewLinterFromAnalyzer(intrange.Analyzer).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
