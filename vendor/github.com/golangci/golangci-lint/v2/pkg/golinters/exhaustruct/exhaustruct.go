package exhaustruct

import (
	exhaustruct "dev.gaijin.team/go/exhaustruct/v4/analyzer"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/internal"
)

func New(settings *config.ExhaustructSettings) *goanalysis.Linter {
	cfg := exhaustruct.Config{}
	if settings != nil {
		cfg.IncludeRx = settings.Include
		cfg.ExcludeRx = settings.Exclude
		cfg.AllowEmpty = settings.AllowEmpty
		cfg.AllowEmptyRx = settings.AllowEmptyRx
		cfg.AllowEmptyReturns = settings.AllowEmptyReturns
		cfg.AllowEmptyDeclarations = settings.AllowEmptyDeclarations
	}

	analyzer, err := exhaustruct.NewAnalyzer(cfg)
	if err != nil {
		internal.LinterLogger.Fatalf("exhaustruct configuration: %v", err)
	}

	return goanalysis.
		NewLinterFromAnalyzer(analyzer).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
