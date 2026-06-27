package modernize

import (
	"slices"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/modernize"
)

func New(settings *config.ModernizeSettings) *goanalysis.Linter {
	var analyzers []*analysis.Analyzer

	if settings == nil {
		analyzers = modernize.Suite
	} else {
		for _, analyzer := range modernize.Suite {
			if slices.Contains(settings.Disable, analyzer.Name) {
				continue
			}

			analyzers = append(analyzers, analyzer)
		}
	}

	return goanalysis.NewLinter(
		"modernize",
		"A suite of analyzers that suggest simplifications to Go code, using modern language and library features.",
		analyzers,
		nil).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
