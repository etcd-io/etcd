package gci

import (
	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/goformatters"
	gcibase "github.com/golangci/golangci-lint/v2/pkg/goformatters/gci"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/internal"
)

const linterName = "gci"

func New(settings *config.GciSettings) *goanalysis.Linter {
	formatter, err := gcibase.New(settings)
	if err != nil {
		internal.LinterLogger.Fatalf("%s: create analyzer: %v", linterName, err)
	}

	return goanalysis.
		NewLinterFromAnalyzer(
			goformatters.NewAnalyzer(
				internal.LinterLogger.Child(linterName),
				"Check if code and import statements are formatted, with additional rules.",
				formatter,
			),
		).
		WithLoadMode(goanalysis.LoadModeSyntax)
}
