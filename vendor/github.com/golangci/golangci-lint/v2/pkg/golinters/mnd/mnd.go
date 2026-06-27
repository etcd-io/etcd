package mnd

import (
	mnd "github.com/tommy-muehle/go-mnd/v2"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.MndSettings) *goanalysis.Linter {
	cfg := map[string]any{}

	if settings != nil {
		if len(settings.Checks) > 0 {
			cfg["checks"] = settings.Checks
		}
		if len(settings.IgnoredNumbers) > 0 {
			cfg["ignored-numbers"] = settings.IgnoredNumbers
		}
		if len(settings.IgnoredFiles) > 0 {
			cfg["ignored-files"] = settings.IgnoredFiles
		}
		if len(settings.IgnoredFunctions) > 0 {
			cfg["ignored-functions"] = settings.IgnoredFunctions
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(mnd.Analyzer).
		WithDesc("An analyzer to detect magic numbers.").
		WithConfig(cfg).
		WithLoadMode(goanalysis.LoadModeSyntax)
}
