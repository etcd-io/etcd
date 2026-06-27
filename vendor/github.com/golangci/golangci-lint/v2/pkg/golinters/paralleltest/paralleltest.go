package paralleltest

import (
	"github.com/kunwardeep/paralleltest/pkg/paralleltest"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.ParallelTestSettings) *goanalysis.Linter {
	var cfg map[string]any

	if settings != nil {
		cfg = map[string]any{
			"i":                     settings.IgnoreMissing,
			"ignoremissingsubtests": settings.IgnoreMissingSubtests,
			"checkcleanup":          settings.CheckCleanup,
		}

		if config.IsGoGreaterThanOrEqual(settings.Go, "1.22") {
			cfg["ignoreloopVar"] = true
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(paralleltest.NewAnalyzer()).
		WithDesc("Detects missing usage of t.Parallel() method in your Go test").
		WithConfig(cfg).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
