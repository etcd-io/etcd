package gofumpt

import (
	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/goformatters"
	gofumptbase "github.com/golangci/golangci-lint/v2/pkg/goformatters/gofumpt"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/internal"
)

func New(settings *config.GoFumptSettings) *goanalysis.Linter {
	return goanalysis.
		NewLinterFromAnalyzer(
			goformatters.NewAnalyzer(
				internal.LinterLogger.Child(gofumptbase.Name),
				"Check if code and import statements are formatted, with additional rules.",
				gofumptbase.New(settings, settings.LangVersion),
			),
		).
		WithLoadMode(goanalysis.LoadModeSyntax)
}
