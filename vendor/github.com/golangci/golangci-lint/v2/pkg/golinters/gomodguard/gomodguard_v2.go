package gomodguard

import (
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/ryancurrah/gomodguard/v2"
	"golang.org/x/tools/go/analysis"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/internal"
	"github.com/golangci/golangci-lint/v2/pkg/lint/linter"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

const linterNameV2 = "gomodguard_v2"

func NewV2(settings *config.GoModGuardv2Settings) *goanalysis.Linter {
	var issues []*goanalysis.Issue
	var mu sync.Mutex

	processorCfg := &gomodguard.Configuration{}
	if settings != nil {
		processorCfg.LocalReplaceDirectives = settings.LocalReplaceDirectives

		for _, allowed := range settings.Allowed {
			rule := gomodguard.AllowedModule{
				Module:    allowed.Module,
				MatchType: gomodguard.MatchType(allowed.MatchType),
				Version:   nil,
				Matcher:   nil,
			}

			if allowed.Version != "" {
				var err error

				rule.Version, err = semver.NewConstraint(allowed.Version)
				if err != nil {
					internal.LinterLogger.Fatalf("gomodguard: invalid constraint: %v", err)
				}
			}

			processorCfg.Allowed = append(processorCfg.Allowed, rule)
		}

		for _, blocked := range settings.Blocked {
			rule := gomodguard.BlockedModule{
				Module:          blocked.Module,
				MatchType:       gomodguard.MatchType(blocked.MatchType),
				Recommendations: blocked.Recommendations,
				Reason:          blocked.Reason,
			}

			if blocked.Version != "" {
				var err error

				rule.Version, err = semver.NewConstraint(blocked.Version)
				if err != nil {
					internal.LinterLogger.Fatalf("gomodguard: invalid constraint: %v", err)
				}
			}

			processorCfg.Blocked = append(processorCfg.Blocked, rule)
		}
	}

	analyzer := &analysis.Analyzer{
		Name: linterNameV2,
		Doc: "Allow and blocklist linter for direct Go module dependencies. " +
			"This is different from depguard where there are different block " +
			"types for example version constraints and module recommendations.",
		Run: goanalysis.DummyRun,
	}

	return goanalysis.NewLinterFromAnalyzer(analyzer).
		WithContextSetter(func(lintCtx *linter.Context) {
			processor, err := gomodguard.NewProcessor(processorCfg)
			if err != nil {
				lintCtx.Log.Warnf("running gomodguard failed: %s: if you are not using go modules "+
					"it is suggested to disable this linter", err)
				return
			}

			analyzer.Run = func(pass *analysis.Pass) (any, error) {
				gomodguardIssues := processor.ProcessFiles(internal.GetGoFileNames(pass))

				mu.Lock()
				defer mu.Unlock()

				for _, gomodguardIssue := range gomodguardIssues {
					issues = append(issues, goanalysis.NewIssue(&result.Issue{
						FromLinter: linterNameV2,
						Pos:        gomodguardIssue.Position,
						Text:       gomodguardIssue.Reason,
					}, pass))
				}

				return nil, nil
			}
		}).
		WithIssuesReporter(func(*linter.Context) []*goanalysis.Issue {
			return issues
		}).
		WithLoadMode(goanalysis.LoadModeSyntax)
}
