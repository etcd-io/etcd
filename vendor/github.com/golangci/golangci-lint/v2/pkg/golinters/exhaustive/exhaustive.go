package exhaustive

import (
	"github.com/nishanths/exhaustive"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.ExhaustiveSettings) *goanalysis.Linter {
	var cfg map[string]any

	if settings != nil {
		cfg = map[string]any{
			exhaustive.CheckFlag:                      settings.Check,
			exhaustive.DefaultSignifiesExhaustiveFlag: settings.DefaultSignifiesExhaustive,
			exhaustive.IgnoreEnumMembersFlag:          settings.IgnoreEnumMembers,
			exhaustive.IgnoreEnumTypesFlag:            settings.IgnoreEnumTypes,
			exhaustive.PackageScopeOnlyFlag:           settings.PackageScopeOnly,
			exhaustive.ExplicitExhaustiveMapFlag:      settings.ExplicitExhaustiveMap,
			exhaustive.ExplicitExhaustiveSwitchFlag:   settings.ExplicitExhaustiveSwitch,
			exhaustive.DefaultCaseRequiredFlag:        settings.DefaultCaseRequired,
			// Should be managed with `linters.exclusions.generated`.
			exhaustive.CheckGeneratedFlag: true,
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(exhaustive.Analyzer).
		WithConfig(cfg).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
