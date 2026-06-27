package iotamixing

import (
	im "github.com/AdminBenni/iota-mixing/pkg/analyzer"
	"github.com/AdminBenni/iota-mixing/pkg/analyzer/flags"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.IotaMixingSettings) *goanalysis.Linter {
	cfg := map[string]any{}

	if settings != nil {
		cfg[flags.ReportIndividualFlagName] = settings.ReportIndividual
	}

	analyzer := im.GetIotaMixingAnalyzer()

	flags.SetupFlags(&analyzer.Flags)

	return goanalysis.
		NewLinterFromAnalyzer(analyzer).
		WithConfig(cfg).
		WithLoadMode(goanalysis.LoadModeSyntax)
}
