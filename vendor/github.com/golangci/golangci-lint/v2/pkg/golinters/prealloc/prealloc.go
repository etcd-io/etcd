package prealloc

import (
	"github.com/alexkohler/prealloc/pkg"
	"golang.org/x/tools/go/analysis"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.PreallocSettings) *goanalysis.Linter {
	return goanalysis.
		NewLinterFromAnalyzer(&analysis.Analyzer{
			Name: "prealloc",
			Doc:  "Find slice declarations that could potentially be pre-allocated",
			Run: func(pass *analysis.Pass) (any, error) {
				pkg.Check(pass, settings.Simple, settings.RangeLoops, settings.ForLoops)

				return nil, nil
			},
		}).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
