package loggercheck

import (
	"github.com/timonwong/loggercheck"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.LoggerCheckSettings) *goanalysis.Linter {
	var opts []loggercheck.Option

	if settings != nil {
		var disable []string
		if !settings.Kitlog {
			disable = append(disable, "kitlog")
		}
		if !settings.Klog {
			disable = append(disable, "klog")
		}
		if !settings.Logr {
			disable = append(disable, "logr")
		}
		if !settings.Slog {
			disable = append(disable, "slog")
		}
		if !settings.Zap {
			disable = append(disable, "zap")
		}

		opts = []loggercheck.Option{
			loggercheck.WithDisable(disable),
			loggercheck.WithRequireStringKey(settings.RequireStringKey),
			loggercheck.WithRules(settings.Rules),
			loggercheck.WithNoPrintfLike(settings.NoPrintfLike),
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(loggercheck.NewAnalyzer(opts...)).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
