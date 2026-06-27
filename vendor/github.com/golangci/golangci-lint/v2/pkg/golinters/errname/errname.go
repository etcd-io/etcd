package errname

import (
	"github.com/Antonboom/errname/pkg/analyzer"

	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New() *goanalysis.Linter {
	return goanalysis.
		NewLinterFromAnalyzer(analyzer.New()).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
