package spancheck

import (
	"github.com/jjti/go-spancheck"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.SpancheckSettings) *goanalysis.Linter {
	cfg := spancheck.NewDefaultConfig()

	if settings != nil {
		if len(settings.Checks) > 0 {
			cfg.EnabledChecks = settings.Checks
		}

		if len(settings.IgnoreCheckSignatures) > 0 {
			cfg.IgnoreChecksSignaturesSlice = settings.IgnoreCheckSignatures
		}

		if len(settings.ExtraStartSpanSignatures) > 0 {
			cfg.StartSpanMatchersSlice = append(cfg.StartSpanMatchersSlice, settings.ExtraStartSpanSignatures...)
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(spancheck.NewAnalyzerWithConfig(cfg)).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
