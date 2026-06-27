package tagliatelle

import (
	"maps"
	"strings"

	"github.com/ldez/tagliatelle"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.TagliatelleSettings) *goanalysis.Linter {
	cfg := tagliatelle.Config{
		Base: tagliatelle.Base{
			Rules: map[string]string{
				"json":   "camel",
				"yaml":   "camel",
				"header": "header",
			},
		},
	}

	if settings != nil {
		maps.Copy(cfg.Rules, settings.Case.Rules)

		cfg.ExtendedRules = toExtendedRules(settings.Case.ExtendedRules)
		cfg.UseFieldName = settings.Case.UseFieldName
		cfg.IgnoredFields = settings.Case.IgnoredFields

		for _, override := range settings.Case.Overrides {
			cfg.Overrides = append(cfg.Overrides, tagliatelle.Overrides{
				Base: tagliatelle.Base{
					Rules:         override.Rules,
					ExtendedRules: toExtendedRules(override.ExtendedRules),
					UseFieldName:  override.UseFieldName,
					IgnoredFields: override.IgnoredFields,
					Ignore:        override.Ignore,
				},
				Package: override.Package,
			})
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(tagliatelle.New(cfg)).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}

func toExtendedRules(src map[string]config.TagliatelleExtendedRule) map[string]tagliatelle.ExtendedRule {
	result := make(map[string]tagliatelle.ExtendedRule, len(src))

	for k, v := range src {
		initialismOverrides := make(map[string]bool, len(v.InitialismOverrides))
		for ki, vi := range v.InitialismOverrides {
			initialismOverrides[strings.ToUpper(ki)] = vi
		}

		result[k] = tagliatelle.ExtendedRule{
			Case:                v.Case,
			ExtraInitialisms:    v.ExtraInitialisms,
			InitialismOverrides: initialismOverrides,
		}
	}

	return result
}
