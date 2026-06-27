package usestdlibvars

import (
	"github.com/sashamelentyev/usestdlibvars/pkg/analyzer"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.UseStdlibVarsSettings) *goanalysis.Linter {
	var cfg map[string]any

	if settings != nil {
		cfg = map[string]any{
			analyzer.ConstantKindFlag:       settings.ConstantKind,
			analyzer.CryptoHashFlag:         settings.CryptoHash,
			analyzer.HTTPMethodFlag:         settings.HTTPMethod,
			analyzer.HTTPStatusCodeFlag:     settings.HTTPStatusCode,
			analyzer.OSDevNullFlag:          false, // Noop because the linter ignore it.
			analyzer.RPCDefaultPathFlag:     settings.DefaultRPCPath,
			analyzer.SQLIsolationLevelFlag:  settings.SQLIsolationLevel,
			analyzer.SyslogPriorityFlag:     false, // Noop because the linter ignore it.
			analyzer.TimeLayoutFlag:         settings.TimeLayout,
			analyzer.TimeMonthFlag:          settings.TimeMonth,
			analyzer.TimeWeekdayFlag:        settings.TimeWeekday,
			analyzer.TLSSignatureSchemeFlag: settings.TLSSignatureScheme,
			analyzer.TimeDateMonthFlag:      settings.TimeDateMonth,
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(analyzer.New()).
		WithConfig(cfg).
		WithLoadMode(goanalysis.LoadModeSyntax)
}
