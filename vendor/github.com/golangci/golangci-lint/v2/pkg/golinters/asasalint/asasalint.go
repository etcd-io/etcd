package asasalint

import (
	"github.com/alingse/asasalint"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/internal"
)

func New(settings *config.AsasalintSettings) *goanalysis.Linter {
	cfg := asasalint.LinterSetting{}
	if settings != nil {
		cfg.Exclude = settings.Exclude
		cfg.NoBuiltinExclusions = !settings.UseBuiltinExclusions

		// Should be managed with `linters.exclusions.rules`.
		cfg.IgnoreTest = false
	}

	analyzer, err := asasalint.NewAnalyzer(cfg)
	if err != nil {
		internal.LinterLogger.Fatalf("asasalint: create analyzer: %v", err)
	}

	return goanalysis.
		NewLinterFromAnalyzer(analyzer).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
