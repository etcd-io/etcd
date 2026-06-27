package nonamedreturns

import (
	"github.com/firefart/nonamedreturns/analyzer"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.NoNamedReturnsSettings) *goanalysis.Linter {
	var cfg map[string]any

	if settings != nil {
		cfg = map[string]any{
			analyzer.FlagReportErrorInDefer: settings.ReportErrorInDefer,
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(analyzer.Analyzer).
		WithConfig(cfg).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
