package zerologlint

import (
	"github.com/ykadowak/zerologlint"

	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New() *goanalysis.Linter {
	return goanalysis.
		NewLinterFromAnalyzer(zerologlint.Analyzer).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
