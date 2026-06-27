package predeclared

import (
	"strings"

	"github.com/nishanths/predeclared/passes/predeclared"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.PredeclaredSettings) *goanalysis.Linter {
	var cfg map[string]any

	if settings != nil {
		cfg = map[string]any{
			predeclared.IgnoreFlag:    strings.Join(settings.Ignore, ","),
			predeclared.QualifiedFlag: settings.Qualified,
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(predeclared.Analyzer).
		WithConfig(cfg).
		WithLoadMode(goanalysis.LoadModeSyntax)
}
