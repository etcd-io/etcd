package unqueryvet

import (
	"github.com/MirrexOne/unqueryvet"
	pkgconfig "github.com/MirrexOne/unqueryvet/pkg/config"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.UnqueryvetSettings) *goanalysis.Linter {
	cfg := pkgconfig.DefaultSettings()

	if settings != nil {
		// IgnoredFiles, Ignore, Severity, and Rules are explicitly ignored.
		cfg.CheckSQLBuilders = settings.CheckSQLBuilders
		cfg.CheckAliasedWildcard = settings.CheckAliasedWildcard
		cfg.CheckStringConcat = settings.CheckStringConcat
		cfg.CheckFormatStrings = settings.CheckFormatStrings
		cfg.CheckStringBuilder = settings.CheckStringBuilder
		cfg.CheckSubqueries = settings.CheckSubqueries
		cfg.N1DetectionEnabled = settings.CheckN1
		cfg.SQLInjectionDetectionEnabled = settings.CheckSQLInjection
		cfg.TxLeakDetectionEnabled = settings.CheckTxLeak
		cfg.IgnoredFunctions = settings.IgnoredFunctions

		for _, rule := range settings.CustomRules {
			// The field Fix is explicitly ignored.
			cfg.CustomRules = append(cfg.CustomRules, pkgconfig.CustomRule{
				ID:       rule.ID,
				Pattern:  rule.Pattern,
				Patterns: rule.Patterns,
				When:     rule.When,
				Message:  rule.Message,
				Severity: "error",
				Action:   rule.Action,
			})
		}

		if len(settings.Allow) > 0 {
			cfg.Allow = settings.Allow
		}

		if len(settings.AllowedPatterns) > 0 {
			cfg.AllowedPatterns = settings.AllowedPatterns
		}

		cfg.SQLBuilders = pkgconfig.SQLBuildersConfig{
			Squirrel:  settings.SQLBuilders.Squirrel,
			GORM:      settings.SQLBuilders.GORM,
			SQLx:      settings.SQLBuilders.SQLx,
			Ent:       settings.SQLBuilders.Ent,
			PGX:       settings.SQLBuilders.PGX,
			Bun:       settings.SQLBuilders.Bun,
			SQLBoiler: settings.SQLBuilders.SQLBoiler,
			Jet:       settings.SQLBuilders.Jet,
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(unqueryvet.NewWithConfig(&cfg)).
		WithLoadMode(goanalysis.LoadModeSyntax)
}
