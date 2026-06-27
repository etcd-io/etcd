package wsl

import (
	"slices"

	wslv4 "github.com/bombsimon/wsl/v4"
	wslv5 "github.com/bombsimon/wsl/v5"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/internal"
)

// Deprecated: use NewV5 instead.
func NewV4(settings *config.WSLv4Settings) *goanalysis.Linter {
	var conf *wslv4.Configuration

	if settings != nil {
		conf = &wslv4.Configuration{
			StrictAppend:                     settings.StrictAppend,
			AllowAssignAndCallCuddle:         settings.AllowAssignAndCallCuddle,
			AllowAssignAndAnythingCuddle:     settings.AllowAssignAndAnythingCuddle,
			AllowMultiLineAssignCuddle:       settings.AllowMultiLineAssignCuddle,
			ForceCaseTrailingWhitespaceLimit: settings.ForceCaseTrailingWhitespaceLimit,
			AllowTrailingComment:             settings.AllowTrailingComment,
			AllowSeparatedLeadingComment:     settings.AllowSeparatedLeadingComment,
			AllowCuddleDeclaration:           settings.AllowCuddleDeclaration,
			AllowCuddleWithCalls:             settings.AllowCuddleWithCalls,
			AllowCuddleWithRHS:               settings.AllowCuddleWithRHS,
			ForceCuddleErrCheckAndAssign:     settings.ForceCuddleErrCheckAndAssign,
			AllowCuddleUsedInBlock:           settings.AllowCuddleUsedInBlock,
			ErrorVariableNames:               settings.ErrorVariableNames,
			ForceExclusiveShortDeclarations:  settings.ForceExclusiveShortDeclarations,
			IncludeGenerated:                 true, // force to true because golangci-lint already have a way to filter generated files.
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(wslv4.NewAnalyzer(conf)).
		WithLoadMode(goanalysis.LoadModeSyntax)
}

// Only used the set YAML struct tags.
type v5YAML struct {
	AllowFirstInBlock bool     `yaml:"allow-first-in-block"`
	AllowWholeBlock   bool     `yaml:"allow-whole-block"`
	BranchMaxLines    int      `yaml:"branch-max-lines,omitempty"`
	CaseMaxLines      int      `yaml:"case-max-lines,omitempty"`
	Enable            []string `yaml:"enable,omitempty"`
	Disable           []string `yaml:"disable,omitempty"`
}

func Migration(old *config.WSLv4Settings) any {
	if old == nil {
		return nil
	}

	cfg := v5YAML{
		AllowFirstInBlock: true,
		AllowWholeBlock:   false,
		BranchMaxLines:    2,
		CaseMaxLines:      old.ForceCaseTrailingWhitespaceLimit,
	}

	if !old.StrictAppend {
		cfg.Disable = append(cfg.Disable, wslv5.CheckAppend.String())
	}

	if old.AllowAssignAndAnythingCuddle {
		cfg.Disable = append(cfg.Disable, wslv5.CheckAssign.String())
	}

	if old.AllowMultiLineAssignCuddle {
		internal.LinterLogger.Warnf("`allow-multiline-assign` is deprecated and always allowed in wsl >= v5")
	}

	if old.AllowTrailingComment {
		internal.LinterLogger.Warnf("`allow-trailing-comment` is deprecated and always allowed in wsl >= v5")
	}

	if old.AllowSeparatedLeadingComment {
		internal.LinterLogger.Warnf("`allow-separated-leading-comment` is deprecated and always allowed in wsl >= v5")
	}

	if old.AllowCuddleDeclaration {
		cfg.Disable = append(cfg.Disable, wslv5.CheckDecl.String())
	}

	if old.ForceCuddleErrCheckAndAssign {
		cfg.Enable = append(cfg.Enable, wslv5.CheckErr.String())
	}

	if old.ForceExclusiveShortDeclarations {
		cfg.Enable = append(cfg.Enable, wslv5.CheckAssignExclusive.String())
	}

	if !old.AllowAssignAndCallCuddle {
		cfg.Enable = append(cfg.Enable, wslv5.CheckAssignExpr.String())
	}

	slices.Sort(cfg.Enable)
	slices.Sort(cfg.Disable)

	return cfg
}
