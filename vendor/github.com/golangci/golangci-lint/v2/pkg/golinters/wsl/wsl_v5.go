package wsl

import (
	"github.com/bombsimon/wsl/v5"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/internal"
)

func NewV5(settings *config.WSLv5Settings) *goanalysis.Linter {
	var conf *wsl.Configuration

	if settings != nil {
		checkSet, err := wsl.NewCheckSet(settings.Default, settings.Enable, settings.Disable)
		if err != nil {
			internal.LinterLogger.Fatalf("wsl: invalid check: %v", err)
		}

		conf = &wsl.Configuration{
			IncludeGenerated:    true, // force to true because golangci-lint already has a way to filter generated files.
			AllowFirstInBlock:   settings.AllowFirstInBlock,
			AllowWholeBlock:     settings.AllowWholeBlock,
			BranchMaxLines:      settings.BranchMaxLines,
			CaseMaxLines:        settings.CaseMaxLines,
			CuddleMaxStatements: settings.CuddleMaxStatements,
			Checks:              checkSet,
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(wsl.NewAnalyzer(conf)).
		WithVersion(5). //nolint:mnd // It's the linter version.
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
