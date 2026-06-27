package maintidx

import (
	"github.com/yagipy/maintidx"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.MaintIdxSettings) *goanalysis.Linter {
	cfg := map[string]any{
		"under": 20,
	}

	if settings != nil {
		cfg["under"] = settings.Under
	}

	return goanalysis.
		NewLinterFromAnalyzer(maintidx.Analyzer).
		WithConfig(cfg).
		WithLoadMode(goanalysis.LoadModeSyntax)
}
