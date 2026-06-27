package testifylint

import (
	"github.com/Antonboom/testifylint/analyzer"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.TestifylintSettings) *goanalysis.Linter {
	var cfg map[string]any

	if settings != nil {
		cfg = map[string]any{
			"enable-all":  settings.EnableAll,
			"disable-all": settings.DisableAll,

			"bool-compare.ignore-custom-types": settings.BoolCompare.IgnoreCustomTypes,
			"formatter.require-f-funcs":        settings.Formatter.RequireFFuncs,
			"formatter.require-string-msg":     settings.Formatter.RequireStringMsg,
			"go-require.ignore-http-handlers":  settings.GoRequire.IgnoreHTTPHandlers,
		}
		if len(settings.EnabledCheckers) > 0 {
			cfg["enable"] = settings.EnabledCheckers
		}
		if len(settings.DisabledCheckers) > 0 {
			cfg["disable"] = settings.DisabledCheckers
		}

		if b := settings.Formatter.CheckFormatString; b != nil {
			cfg["formatter.check-format-string"] = *b
		}
		if p := settings.ExpectedActual.ExpVarPattern; p != "" {
			cfg["expected-actual.pattern"] = p
		}
		if p := settings.RequireError.FnPattern; p != "" {
			cfg["require-error.fn-pattern"] = p
		}
		if m := settings.SuiteExtraAssertCall.Mode; m != "" {
			cfg["suite-extra-assert-call.mode"] = m
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(analyzer.New()).
		WithConfig(cfg).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
